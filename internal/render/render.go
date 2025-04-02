// Copyright 2025 Upbound Inc.
// All rights reserved

// Package render contains functions for composition rendering
package render

import (
	"bytes"
	"context"
	"fmt"
	"runtime"

	"github.com/google/go-containerregistry/pkg/name"
	v1cache "github.com/google/go-containerregistry/pkg/v1/cache"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	xprender "github.com/crossplane/crossplane/cmd/crank/render"
	"github.com/crossplane/crossplane/xcrd"

	"github.com/upbound/up/cmd/up/project/common"
	"github.com/upbound/up/internal/async"
	icrd "github.com/upbound/up/internal/crd"
	"github.com/upbound/up/internal/oci/cache"
	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/xpkg"
	"github.com/upbound/up/internal/xpkg/dep/manager"
	"github.com/upbound/up/internal/xpkg/functions"
	"github.com/upbound/up/internal/xpkg/schemarunner"
	"github.com/upbound/up/internal/yaml"
	projectv1alpha1 "github.com/upbound/up/pkg/apis/project/v1alpha1"
)

// Options defines the configuration for rendering.
type Options struct {
	Project *projectv1alpha1.Project
	ProjFS  afero.Fs

	IncludeFullXR          bool
	IncludeFunctionResults bool
	IncludeContext         bool

	CompositeResource   string
	Composition         string
	XRD                 string
	FunctionCredentials string
	ObservedResources   string
	ExtraResources      string

	ContextFiles  map[string]string
	ContextValues map[string]string
	Concurrency   uint

	ImageResolver manager.ImageResolver
}

// FunctionOptions defines the configuration for building embedded functions.
type FunctionOptions struct {
	Project *projectv1alpha1.Project
	ProjFS  afero.Fs

	Concurrency uint

	NoBuildCache       bool
	BuildCacheDir      string
	ImageResolver      manager.ImageResolver
	FunctionIdentifier functions.Identifier
	SchemaRunner       schemarunner.SchemaRunner
	DependecyManager   *manager.Manager
	EventChannel       async.EventChannel
}

// Render executes the rendering logic and returns YAML output as a string.
func Render(ctx context.Context, log logging.Logger, embeddedFunctions []pkgv1.Function, opts Options) (string, error) { //nolint:gocognit // render logic
	xr, err := xprender.LoadCompositeResource(opts.ProjFS, opts.CompositeResource)
	if err != nil {
		return "", errors.Wrapf(err, "cannot load composite resource from %q", opts.CompositeResource)
	}

	comp, err := xprender.LoadComposition(opts.ProjFS, opts.Composition)
	if err != nil {
		return "", errors.Wrapf(err, "cannot load Composition from %q", opts.Composition)
	}

	expected := comp.Spec.CompositeTypeRef
	actual := xr.GetReference()
	if expected.APIVersion != actual.APIVersion || expected.Kind != actual.Kind {
		return "", errors.Errorf(
			"CompositeResource %s.%s does not match Composition compositeTypeRef %s.%s",
			actual.Kind, actual.APIVersion,
			expected.Kind, expected.APIVersion,
		)
	}

	// Load XRD and apply default values if needed
	if opts.XRD != "" {
		xrd, err := loadXRD(opts.ProjFS, opts.XRD)
		if err != nil {
			return "", errors.Wrapf(err, "cannot load XRD from %q", opts.XRD)
		}
		crd, err := xcrd.ForCompositeResource(xrd)
		if err != nil {
			return "", errors.Wrapf(err, "cannot derive composite CRD from XRD %q", xrd.GetName())
		}
		if err := icrd.DefaultValues(xr.UnstructuredContent(), *crd); err != nil {
			return "", errors.Wrapf(err, "cannot default values for XR %q", xr.GetName())
		}
	}

	// Load function credentials
	var fcreds []corev1.Secret
	if opts.FunctionCredentials != "" {
		fcreds, err = xprender.LoadCredentials(opts.ProjFS, opts.FunctionCredentials)
		if err != nil {
			return "", errors.Wrapf(err, "cannot load secrets from %q", opts.FunctionCredentials)
		}
	}

	// Load observed and extra resources
	var ors []composed.Unstructured
	if opts.ExtraResources != "" {
		ors, err = xprender.LoadObservedResources(opts.ProjFS, opts.ObservedResources)
		if err != nil {
			return "", errors.Wrapf(err, "cannot load observed composed resources from %q", opts.ObservedResources)
		}
	}

	var ers []unstructured.Unstructured
	if opts.ExtraResources != "" {
		ers, err = xprender.LoadExtraResources(opts.ProjFS, opts.ExtraResources)
		if err != nil {
			return "", errors.Wrapf(err, "cannot load extra resources from %q", opts.ExtraResources)
		}
	}

	// Load context values
	fctx := make(map[string][]byte)
	for k, filename := range opts.ContextFiles {
		v, err := afero.ReadFile(opts.ProjFS, filename)
		if err != nil {
			return "", errors.Wrapf(err, "cannot read context value for key %q", k)
		}
		fctx[k] = v
	}
	for k, v := range opts.ContextValues {
		fctx[k] = []byte(v)
	}

	// Load additional functions
	fns, err := loadFunctions(ctx, opts.Project, opts.ImageResolver)
	if err != nil {
		return "", errors.Wrap(err, "cannot load functions from project")
	}
	fns = append(fns, embeddedFunctions...)

	// Perform rendering
	out, err := xprender.Render(ctx, log, xprender.Inputs{
		CompositeResource:   xr,
		Composition:         comp,
		Functions:           fns,
		FunctionCredentials: fcreds,
		ObservedResources:   ors,
		ExtraResources:      ers,
		Context:             fctx,
	})
	if err != nil {
		return "", errors.Wrap(err, "cannot render composite resource")
	}

	// Serialize output to YAML
	s := json.NewSerializerWithOptions(json.DefaultMetaFactory, nil, nil, json.SerializerOptions{Yaml: true})
	var result string
	result += "---\n"

	// If IncludeFullXR is set, retain full composite resource details
	if opts.IncludeFullXR {
		xrSpec, err := fieldpath.Pave(xr.Object).GetValue("spec")
		if err != nil {
			return "", errors.Wrapf(err, "cannot get composite resource spec")
		}

		if err := fieldpath.Pave(out.CompositeResource.Object).SetValue("spec", xrSpec); err != nil {
			return "", errors.Wrapf(err, "cannot set composite resource spec")
		}

		xrMeta, err := fieldpath.Pave(xr.Object).GetValue("metadata")
		if err != nil {
			return "", errors.Wrapf(err, "cannot get composite resource metadata")
		}

		if err := fieldpath.Pave(out.CompositeResource.Object).SetValue("metadata", xrMeta); err != nil {
			return "", errors.Wrapf(err, "cannot set composite resource metadata")
		}
	}

	// Encode CompositeResource
	var buffer bytes.Buffer
	if err := s.Encode(out.CompositeResource, &buffer); err != nil {
		return "", errors.Wrap(err, "failed to encode composite resource to YAML")
	}
	result += buffer.String()

	// Encode ComposedResources
	for _, res := range out.ComposedResources {
		result += "---\n"
		buffer.Reset()
		if err := s.Encode(&res, &buffer); err != nil {
			return "", errors.Wrap(err, "failed to encode composed resource to YAML")
		}
		result += buffer.String()
	}

	// Encode FunctionResults if needed
	if opts.IncludeFunctionResults {
		for _, res := range out.Results {
			result += "---\n"
			buffer.Reset()
			if err := s.Encode(&res, &buffer); err != nil {
				return "", errors.Wrap(err, "failed to encode function result to YAML")
			}
			result += buffer.String()
		}
	}

	// Encode Context if needed
	if opts.IncludeContext {
		result += "---\n"
		buffer.Reset()
		if err := s.Encode(out.Context, &buffer); err != nil {
			return "", errors.Wrap(err, "failed to encode context to YAML")
		}
		result += buffer.String()
	}

	return result, nil
}

func loadXRD(fs afero.Fs, file string) (*apiextensionsv1.CompositeResourceDefinition, error) {
	y, err := afero.ReadFile(fs, file)
	if err != nil {
		return nil, errors.Wrap(err, "cannot read XRD file")
	}
	xrd := &apiextensionsv1.CompositeResourceDefinition{}
	return xrd, errors.Wrap(yaml.Unmarshal(y, xrd), "cannot unmarshal XRD YAML")
}

// embeddedFunctionsToDaemon loads each compatible image in the ImageTagMap into the Docker daemon.
func embeddedFunctionsToDaemon(imageMap project.ImageTagMap) ([]pkgv1.Function, error) {
	functions := make([]pkgv1.Function, 0, len(imageMap))

	for tag, img := range imageMap {
		platformInfo, err := img.ConfigFile()
		if err != nil {
			return nil, errors.Wrapf(err, "error getting platform info for image %s", tag)
		}

		if platformInfo.Architecture != runtime.GOARCH {
			continue
		}

		// Push the image directly to the daemon
		if _, err := daemon.Write(tag, img); err != nil {
			return nil, errors.Wrapf(err, "error pushing image %s to daemon", tag)
		}

		f := pkgv1.Function{
			ObjectMeta: metav1.ObjectMeta{
				// align name with functionRef.name in composition
				Name: xpkg.ToDNSLabel(tag.Context().RepositoryStr()),
			},
			Spec: pkgv1.FunctionSpec{
				PackageSpec: pkgv1.PackageSpec{
					// set correct local image with tag
					Package: tag.Name(),
				},
			},
		}

		functions = append(functions, f)
	}

	return functions, nil
}

// BuildEmbeddedFunctionsLocalDaemon build and push to local deamon.
func BuildEmbeddedFunctionsLocalDaemon(ctx context.Context, opts FunctionOptions) ([]pkgv1.Function, error) {
	b := project.NewBuilder(
		project.BuildWithMaxConcurrency(opts.Concurrency),
		project.BuildWithFunctionIdentifier(opts.FunctionIdentifier),
		project.BuildWithSchemaRunner(opts.SchemaRunner),
	)

	imgMap, err := b.Build(ctx, opts.Project, opts.ProjFS,
		project.BuildWithEventChannel(opts.EventChannel),
		project.BuildWithImageLabels(common.ImageLabels(opts)),
		project.BuildWithDependencyManager(opts.DependecyManager),
	)
	if err != nil {
		return nil, err
	}

	if !opts.NoBuildCache {
		cch := cache.NewValidatingCache(v1cache.NewFilesystemCache(opts.BuildCacheDir))
		for tag, img := range imgMap {
			imgMap[tag] = v1cache.Image(img, cch)
		}
	}

	stage := "Pushing embedded functions to local daemon"
	opts.EventChannel.SendEvent(stage, async.EventStatusStarted)
	efns, err := embeddedFunctionsToDaemon(imgMap)
	if err != nil {
		opts.EventChannel.SendEvent(stage, async.EventStatusFailure)
		return nil, errors.Wrap(err, "unable to push to local docker daemon")
	}
	opts.EventChannel.SendEvent(stage, async.EventStatusSuccess)

	return efns, nil
}

// LoadFunctions loads functions from a project's DependsOn list.
func loadFunctions(ctx context.Context, proj *projectv1alpha1.Project, r manager.ImageResolver) ([]pkgv1.Function, error) {
	functions := make([]pkgv1.Function, 0, len(proj.Spec.DependsOn))

	for _, dep := range proj.Spec.DependsOn {
		if dep.Function == nil {
			continue
		}

		// Convert function dependency
		convertedDep, ok := manager.ConvertToV1beta1(dep)
		if !ok {
			return nil, errors.Errorf("failed to convert dependency in %s", *dep.Function)
		}

		// Resolve tag for function
		version, err := r.ResolveTag(ctx, convertedDep)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to resolve tag for function %s", *dep.Function)
		}

		// Parse function name
		functionRepo, err := name.NewRepository(*dep.Function, name.StrictValidation)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse function name reference for %s", *dep.Function)
		}

		// Create function package manifest
		f := pkgv1.Function{
			ObjectMeta: metav1.ObjectMeta{
				Name: xpkg.ToDNSLabel(functionRepo.RepositoryStr()),
			},
			Spec: pkgv1.FunctionSpec{
				PackageSpec: pkgv1.PackageSpec{
					Package: fmt.Sprintf("%s:%s", *dep.Function, version),
				},
			},
		}
		functions = append(functions, f)
	}

	return functions, nil
}
