// Copyright 2025 Upbound Inc.
// All rights reserved

package project

import (
	"context"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/spf13/afero"
	"golang.org/x/sync/errgroup"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/parser"
	xpv1 "github.com/crossplane/crossplane/v2/apis/apiextensions/v1"
	extv1alpha1 "github.com/crossplane/crossplane/v2/apis/apiextensions/v1alpha1"
	xpv2 "github.com/crossplane/crossplane/v2/apis/apiextensions/v2"
	xpv1alpha1 "github.com/crossplane/crossplane/v2/apis/ops/v1alpha1"
	xpmetav1 "github.com/crossplane/crossplane/v2/apis/pkg/meta/v1"
	xpkgv1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"

	"github.com/upbound/up/internal/async"
	"github.com/upbound/up/internal/schemas/manager"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/xpkg"
	"github.com/upbound/up/internal/xpkg/functions"
	"github.com/upbound/up/internal/xpkg/mutators"
	"github.com/upbound/up/internal/xpkg/parser/examples"
	"github.com/upbound/up/internal/xpkg/parser/schema"
	pyaml "github.com/upbound/up/internal/xpkg/parser/yaml"
	"github.com/upbound/up/pkg/apis"
	"github.com/upbound/up/pkg/apis/project/v2alpha1"
)

const (
	// ConfigurationTag is the tag used for the configuration image in the built
	// package.
	ConfigurationTag = "configuration"
)

// ImageTagMap is a map of container image tags to images.
type ImageTagMap map[name.Tag]v1.Image

// Builder is able to build a project into a set of packages.
type Builder interface {
	// Build builds a project into a set of packages. It returns a map
	// containing images that were built from the project. The returned map will
	// always include one image with the ConfigurationTag, which is the
	// configuration package built from the APIs found in the project.
	Build(ctx context.Context, upCtx *upbound.Context, project *v2alpha1.Project, projectFS afero.Fs, opts ...BuildOption) (ImageTagMap, error)
}

// BuilderOption configures a builder.
type BuilderOption func(b *realBuilder)

// BuildWithFunctionIdentifier sets the function identifier that will be used to
// find function builders for any functions in a project.
func BuildWithFunctionIdentifier(i functions.Identifier) BuilderOption {
	return func(b *realBuilder) {
		b.functionIdentifier = i
	}
}

// BuildWithMaxConcurrency sets the maximum concurrency for building embedded
// functions.
func BuildWithMaxConcurrency(n uint) BuilderOption {
	return func(b *realBuilder) {
		b.maxConcurrency = n
	}
}

// BuildOption configures a build.
type BuildOption func(o *buildOptions)

type buildOptions struct {
	eventChan       async.EventChannel
	imageLabels     map[string]string
	depManager      *DependencyManager
	projectBasePath string
}

// BuildWithEventChannel provides a channel to which progress updates will be
// written during the build. It is the caller's responsibility to manage the
// lifecycle of this channel.
func BuildWithEventChannel(ch async.EventChannel) BuildOption {
	return func(o *buildOptions) {
		o.eventChan = ch
	}
}

// BuildWithImageLabels provides labels that will be added to all images after
// they are built.
func BuildWithImageLabels(labels map[string]string) BuildOption {
	return func(o *buildOptions) {
		o.imageLabels = labels
	}
}

// BuildWithDependencyManager provides a dependency manager to use for
// dependency resolution during build.
func BuildWithDependencyManager(m *DependencyManager) BuildOption {
	return func(o *buildOptions) {
		o.depManager = m
	}
}

// BuildWithProjectBasePath sets the real on-disk base path of the project. This
// path will be uesd for following symlinks. If not set it will be inferred from
// the project FS, which works only when the project FS is an afero.BasePathFs.
func BuildWithProjectBasePath(path string) BuildOption {
	return func(o *buildOptions) {
		o.projectBasePath = path
	}
}

type realBuilder struct {
	functionIdentifier functions.Identifier
	maxConcurrency     uint
}

// Build implements the Builder interface.
func (b *realBuilder) Build(ctx context.Context, upCtx *upbound.Context, project *v2alpha1.Project, projectFS afero.Fs, opts ...BuildOption) (ImageTagMap, error) {
	os := &buildOptions{}
	for _, opt := range opts {
		opt(os)
	}

	// Scaffold a configuration based on the metadata in the project. Later
	// we'll add any embedded functions we build to the dependencies.
	cfg := &xpmetav1.Configuration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: xpmetav1.SchemeGroupVersion.String(),
			Kind:       xpmetav1.ConfigurationKind,
		},
		ObjectMeta: cfgMetaFromProject(project),
		Spec: xpmetav1.ConfigurationSpec{
			MetaSpec: xpmetav1.MetaSpec{
				Crossplane: project.Spec.Crossplane,
				DependsOn:  project.Spec.DependsOn,
			},
		},
	}

	// If there's no Crossplane constraint specified, default to v2. This is the
	// default for v2 projects; if we're dealing with a v1 project on-disk, we
	// will have filled in the crossplane constraint before converting it to v2.
	if cfg.Spec.Crossplane == nil || cfg.Spec.Crossplane.Version == "" {
		cfg.Spec.Crossplane = &xpmetav1.CrossplaneConstraints{
			Version: ">=v2.0.0-rc.0",
		}
	}

	functionsSource := afero.NewBasePathFs(projectFS, project.Spec.Paths.Functions)
	// By default we search the whole project directory except our specified
	// paths.
	apisSource := projectFS
	apiExcludes := []string{
		project.Spec.Paths.Examples,
		project.Spec.Paths.Functions,
		project.Spec.Paths.Operations,
	}
	if project.Spec.Paths.APIs != "/" {
		apisSource = afero.NewBasePathFs(projectFS, project.Spec.Paths.APIs)
		apiExcludes = []string{}
	}

	// Not all projects have operations; ignore them if not present.
	operationsSource := afero.NewMemMapFs()
	opsExist, err := afero.DirExists(projectFS, project.Spec.Paths.Operations)
	if err != nil {
		return nil, err
	}
	if opsExist {
		operationsSource = afero.NewBasePathFs(projectFS, project.Spec.Paths.Operations)
	}

	// Collect resources (XRDs, MRAPs, compositions, and operations).
	packageFS := afero.NewMemMapFs()
	statusStage := "Collecting resources"
	os.eventChan.SendEvent(statusStage, async.EventStatusStarted)

	apiGVKs := []string{
		xpv1.CompositeResourceDefinitionGroupVersionKind.String(),
		xpv2.CompositeResourceDefinitionGroupVersionKind.String(),
		xpv1.CompositionGroupVersionKind.String(),
		extv1alpha1.ManagedResourceActivationPolicyGroupVersionKind.String(),
	}
	if err := collectResources(packageFS, apisSource, apiGVKs, apiExcludes); err != nil {
		os.eventChan.SendEvent(statusStage, async.EventStatusFailure)
		return nil, err
	}

	opsGVKs := []string{
		xpv1alpha1.OperationGroupVersionKind.String(),
		xpv1alpha1.WatchOperationGroupVersionKind.String(),
		xpv1alpha1.CronOperationGroupVersionKind.String(),
	}
	if err := collectResources(packageFS, operationsSource, opsGVKs, nil); err != nil {
		os.eventChan.SendEvent(statusStage, async.EventStatusFailure)
		return nil, err
	}

	os.eventChan.SendEvent(statusStage, async.EventStatusSuccess)

	// Generate schemas for our APIs.
	statusStage = "Generating language schemas"
	os.eventChan.SendEvent(statusStage, async.EventStatusStarted)
	schemas, err := os.depManager.SchemaManager().Generate(ctx, manager.NewFSSource(apisSource))
	if err != nil {
		os.eventChan.SendEvent(statusStage, async.EventStatusFailure)
		return nil, errors.Wrap(err, "failed to generate language schemas")
	}
	if err := apis.GenerateSchema(ctx, os.depManager.schemas); err != nil {
		os.eventChan.SendEvent(statusStage, async.EventStatusFailure)
		return nil, errors.Wrap(err, "failed to generate language schemas for Upbound meta APIs")
	}
	os.eventChan.SendEvent(statusStage, async.EventStatusSuccess)

	// Check that we have all the dependencies in the cache for function
	// building
	statusStage = "Checking dependencies"
	os.eventChan.SendEvent(statusStage, async.EventStatusStarted)
	if err := os.depManager.AddAll(ctx, project.Spec.DependsOn...); err != nil {
		os.eventChan.SendEvent(statusStage, async.EventStatusFailure)
		return nil, err
	}
	if err := os.depManager.AddAllAPIDependencies(ctx, project.Spec.APIDependencies); err != nil {
		os.eventChan.SendEvent(statusStage, async.EventStatusFailure)
		return nil, err
	}
	os.eventChan.SendEvent(statusStage, async.EventStatusSuccess)

	// Find and build embedded functions. This has to come after schema
	// generation because functions may depend on the generated schemas.
	statusStage = "Building functions"
	os.eventChan.SendEvent(statusStage, async.EventStatusStarted)
	imgMap, deps, err := b.buildFunctions(ctx, upCtx, functionsSource, project, os.projectBasePath)
	if err != nil {
		os.eventChan.SendEvent(statusStage, async.EventStatusFailure)
		return nil, err
	}
	// Add embedded function dependencies to the configuration.
	cfg.Spec.DependsOn = append(cfg.Spec.DependsOn, deps...)
	os.eventChan.SendEvent(statusStage, async.EventStatusSuccess)

	// Add the package metadata to the collected composites.
	statusStage = "Building configuration package"
	os.eventChan.SendEvent(statusStage, async.EventStatusStarted)
	defer func() {
		if err != nil {
			os.eventChan.SendEvent(statusStage, async.EventStatusFailure)
		} else {
			os.eventChan.SendEvent(statusStage, async.EventStatusSuccess)
		}
	}()

	y, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal package metadata")
	}
	err = afero.WriteFile(packageFS, "/crossplane.yaml", y, 0o644)
	if err != nil {
		return nil, errors.Wrap(err, "failed to write package metadata")
	}

	// Build the configuration package from the constructed filesystem.
	pp, err := pyaml.New()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create parser")
	}
	builder := xpkg.New(
		parser.NewFsBackend(packageFS, parser.FsDir("/")),
		nil,
		parser.NewFsBackend(afero.NewBasePathFs(projectFS, project.Spec.Paths.Examples),
			parser.FsDir("/"),
			parser.FsFilters(parser.SkipNotYAML()),
		),
		nil, // Helm backend is not used here (or not supported yet).
		pp,
		examples.New(),
	)

	img, _, err := builder.Build(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build package")
	}

	if os.imageLabels != nil {
		img, err = addLabels(img, os.imageLabels)
		if err != nil {
			return nil, errors.Wrapf(err, "failed add labels to package")
		}
	}

	img, err = addSchemaLayers(img, schemas)
	if err != nil {
		return nil, errors.Wrap(err, "failed to add schema layers to package")
	}

	// Write out the packages to a file, which can be consumed by up project
	// push.
	imgTag, err := name.NewTag(fmt.Sprintf("%s:%s", project.Spec.Repository, ConfigurationTag))
	if err != nil {
		return nil, errors.Wrap(err, "failed to construct image tag")
	}
	imgMap[imgTag] = img

	return imgMap, nil
}

// buildFunctions builds the embedded functions found in directories at the top
// level of the provided filesystem. The resulting images are returned in a map
// where the keys are their tags, suitable for writing to a file with
// go-containerregistry's `tarball.MultiWrite`.
func (b *realBuilder) buildFunctions(ctx context.Context, upCtx *upbound.Context, fromFS afero.Fs, project *v2alpha1.Project, basePath string) (ImageTagMap, []xpmetav1.Dependency, error) {
	var (
		imgMap = make(map[name.Tag]v1.Image)
		imgMu  sync.Mutex
	)

	infos, err := afero.ReadDir(fromFS, "/")
	switch {
	case os.IsNotExist(err):
		// There are no functions.
		return imgMap, nil, nil
	case err != nil:
		return nil, nil, errors.Wrap(err, "failed to list functions directory")
	}

	fnDirs := make([]string, 0, len(infos))
	for _, info := range infos {
		if info.IsDir() {
			fnDirs = append(fnDirs, info.Name())
		}
	}

	deps := make([]xpmetav1.Dependency, len(fnDirs))
	eg, ctx := errgroup.WithContext(ctx)

	// Semaphore to limit the number of functions we build in parallel.
	sem := make(chan struct{}, b.maxConcurrency)
	for i, fnName := range fnDirs {
		eg.Go(func() error {
			sem <- struct{}{}
			defer func() {
				<-sem
			}()

			fnRepo := fmt.Sprintf("%s_%s", project.Spec.Repository, fnName)
			fnFS := afero.NewBasePathFs(fromFS, fnName)
			fnBasePath := ""
			if basePath != "" {
				fnBasePath = filepath.Join(basePath, project.Spec.Paths.Functions, fnName)
			}
			imgs, err := b.buildFunction(ctx, upCtx, fnFS, project, fnName, fnBasePath)
			if err != nil {
				return errors.Wrapf(err, "failed to build function %q", fnName)
			}

			// Construct an index so we know the digest for the dependency. This
			// index will be reproduced when we push the image.
			idx, imgs, err := xpkg.BuildIndex(imgs...)
			if err != nil {
				return errors.Wrapf(err, "failed to construct index for function image %q", fnName)
			}
			dgst, err := idx.Digest()
			if err != nil {
				return errors.Wrapf(err, "failed to get index digest for function image %q", fnName)
			}
			deps[i] = xpmetav1.Dependency{
				APIVersion: ptr.To(xpkgv1.FunctionGroupVersionKind.GroupVersion().String()),
				Kind:       ptr.To(xpkgv1.FunctionKind),
				Package:    &fnRepo,
				Version:    dgst.String(),
			}

			for _, img := range imgs {
				cfg, err := img.ConfigFile()
				if err != nil {
					return errors.Wrapf(err, "failed to get config for function image %q", fnName)
				}

				tag := fmt.Sprintf("%s:%s", fnRepo, cfg.Architecture)
				imgTag, err := name.NewTag(tag)
				if err != nil {
					return errors.Wrapf(err, "failed to construct tag for function image %q", fnName)
				}
				imgMu.Lock()
				imgMap[imgTag] = img
				imgMu.Unlock()
			}

			return nil
		})
	}

	err = eg.Wait()
	if err != nil {
		return nil, nil, err
	}

	return imgMap, deps, nil
}

// buildFunction builds images for a single function whose source resides in the
// given filesystem. One image will be returned for each architecture specified
// in the project.
func (b *realBuilder) buildFunction(ctx context.Context, upCtx *upbound.Context, fromFS afero.Fs, project *v2alpha1.Project, fnName string, basePath string) ([]v1.Image, error) { //nolint:gocyclo // Factoring anything out here would be unnatural.
	fn := &xpmetav1.Function{
		TypeMeta: metav1.TypeMeta{
			APIVersion: xpmetav1.SchemeGroupVersion.String(),
			Kind:       xpmetav1.FunctionKind,
		},
		ObjectMeta: fnMetaFromProject(project, fnName),
		Spec: xpmetav1.FunctionSpec{
			MetaSpec: xpmetav1.MetaSpec{
				// TODO(adamwg): Ideally, we'd know whether the function is
				// being used as an operation function or a composition function
				// and set the capabilities accordingly. We could figure this
				// out by looking at all the operations and compositions in the
				// project and mapping which embedded functions they call. For
				// now, though, there's little harm in supporting both for all
				// functions, and it's the simplest thing to implement.
				Capabilities: []string{
					xpmetav1.FunctionCapabilityComposition,
					xpmetav1.FunctionCapabilityOperation,
				},
			},
		},
	}
	metaFS := afero.NewMemMapFs()
	y, err := yaml.Marshal(fn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal function metadata")
	}
	err = afero.WriteFile(metaFS, "/crossplane.yaml", y, 0o644)
	if err != nil {
		return nil, errors.Wrap(err, "failed to write function metadata")
	}

	// Note there's no way to configure the location of examples in an embedded
	// function. If we start supporting projects as embedded functions we should
	// probably change this, but for now this is good enough.
	examplesParser := parser.NewEchoBackend("")
	examplesExist, err := afero.IsDir(fromFS, "/examples")
	switch {
	case err == nil, os.IsNotExist(err):
		// Check examplesExist to determine whether to parse examples.
	default:
		return nil, errors.Wrap(err, "failed to check for examples")
	}
	if examplesExist {
		examplesParser = parser.NewFsBackend(fromFS,
			parser.FsDir("/examples"),
			parser.FsFilters(parser.SkipNotYAML()),
		)
	}

	pp, err := pyaml.New()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create parser")
	}
	builder := xpkg.New(
		parser.NewFsBackend(metaFS, parser.FsDir("/")),
		nil,
		examplesParser,
		nil, // Helm backend is not used here (or not supported yet).
		pp,
		examples.New(),
	)

	fnBuilder, err := b.functionIdentifier.Identify(fromFS, upCtx, project.Spec.ImageConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find a builder")
	}

	// Resolve the real absolute path to the function directory if
	// possible. This is required for following symlinks in the function
	// directory.
	if bfs, ok := fromFS.(*afero.BasePathFs); ok && basePath == "" {
		basePath = afero.FullBaseFsPath(bfs, ".")
	}

	runtimeImages, err := fnBuilder.Build(ctx, fromFS, project.Spec.Architectures, basePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build runtime images")
	}

	pkgImages := make([]v1.Image, 0, len(runtimeImages))

	for _, img := range runtimeImages {
		pkgImage, _, err := builder.Build(ctx, xpkg.WithController(img))
		if err != nil {
			return nil, errors.Wrap(err, "failed to build function package")
		}
		pkgImages = append(pkgImages, pkgImage)
	}

	return pkgImages, nil
}

func collectResources(toFS afero.Fs, fromFS afero.Fs, gvks []string, exclude []string) error {
	return afero.Walk(fromFS, "/", func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		for _, excl := range exclude {
			if strings.HasPrefix(path, excl) {
				return filepath.SkipDir
			}
		}

		if info.IsDir() {
			return nil
		}
		// Ignore files without yaml extensions.
		ext := filepath.Ext(path)
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		var u metav1.TypeMeta
		bs, err := afero.ReadFile(fromFS, path)
		if err != nil {
			return errors.Wrapf(err, "failed to read file %q", path)
		}
		err = yaml.Unmarshal(bs, &u)
		if err != nil {
			return errors.Wrapf(err, "failed to parse file %q", path)
		}

		if !slices.Contains(gvks, u.GroupVersionKind().String()) {
			return nil
		}

		// Copy the file into the package FS.
		err = afero.WriteFile(toFS, path, bs, 0o644)
		if err != nil {
			return errors.Wrapf(err, "failed to write file %q to package", path)
		}

		return nil
	})
}

func cfgMetaFromProject(proj *v2alpha1.Project) metav1.ObjectMeta {
	meta := proj.ObjectMeta.DeepCopy()

	if meta.Annotations == nil {
		meta.Annotations = make(map[string]string)
	}

	meta.Annotations["meta.crossplane.io/maintainer"] = proj.Spec.Maintainer
	meta.Annotations["meta.crossplane.io/source"] = proj.Spec.Source
	meta.Annotations["meta.crossplane.io/license"] = proj.Spec.License
	meta.Annotations["meta.crossplane.io/description"] = proj.Spec.Description
	meta.Annotations["meta.crossplane.io/readme"] = proj.Spec.Readme

	maps.Copy(meta.Annotations, proj.Spec.Annotations)

	return *meta
}

func fnMetaFromProject(proj *v2alpha1.Project, fnName string) metav1.ObjectMeta {
	meta := proj.ObjectMeta.DeepCopy()

	meta.Name = fmt.Sprintf("%s-%s", meta.Name, fnName)

	if meta.Annotations == nil {
		meta.Annotations = make(map[string]string)
	}

	meta.Annotations["meta.crossplane.io/maintainer"] = proj.Spec.Maintainer
	meta.Annotations["meta.crossplane.io/source"] = proj.Spec.Source
	meta.Annotations["meta.crossplane.io/license"] = proj.Spec.License
	meta.Annotations["meta.crossplane.io/description"] = fmt.Sprintf("Function %s from project %s", fnName, proj.Name)

	maps.Copy(meta.Annotations, proj.Spec.Annotations)

	return *meta
}

func addLabels(img v1.Image, labels map[string]string) (v1.Image, error) {
	cfgFile, err := img.ConfigFile()
	if err != nil {
		return nil, errors.Wrap(err, "error getting config file")
	}

	if cfgFile.Config.Labels == nil {
		cfgFile.Config.Labels = make(map[string]string)
	}

	maps.Copy(cfgFile.Config.Labels, labels)

	// Mutate the image to include the updated configuration with labels
	updatedImg, err := mutate.ConfigFile(img, cfgFile)
	if err != nil {
		return nil, errors.Wrap(err, "error updating config file with labels")
	}

	return updatedImg, nil
}

func addSchemaLayers(img v1.Image, schemas map[string]afero.Fs) (v1.Image, error) {
	// Add schema layers to the package.
	imgCfgFile, err := img.ConfigFile()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read config file from image")
	}
	cfg := imgCfgFile.Config

	for lang, schemaFS := range schemas {
		p := schema.New(schemaFS, ".", xpkg.StreamFileMode)
		mut := mutators.NewSchemaMutator(p, fmt.Sprintf("schema.%s", lang))

		img, cfg, err = mut.Mutate(img, cfg)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to add schema layer for language %s", lang)
		}
	}

	img, err = mutate.Config(img, cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to mutate config for image")
	}

	return img, nil
}

// NewBuilder returns a new project builder.
func NewBuilder(opts ...BuilderOption) Builder {
	b := &realBuilder{
		functionIdentifier: functions.DefaultIdentifier,
		maxConcurrency:     8,
	}

	for _, opt := range opts {
		opt(b)
	}

	return b
}
