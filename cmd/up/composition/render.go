// Copyright 2024 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package composition contains functions for local composition rendering
package composition

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/alecthomas/kong"
	"github.com/google/go-containerregistry/pkg/name"
	v1cache "github.com/google/go-containerregistry/pkg/v1/cache"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/pterm/pterm"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

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
	"github.com/upbound/up/internal/config"
	icrd "github.com/upbound/up/internal/crd"
	"github.com/upbound/up/internal/oci/cache"
	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/internal/xpkg"
	xcache "github.com/upbound/up/internal/xpkg/dep/cache"
	"github.com/upbound/up/internal/xpkg/dep/manager"
	"github.com/upbound/up/internal/xpkg/dep/resolver/image"
	"github.com/upbound/up/internal/xpkg/functions"
	"github.com/upbound/up/internal/xpkg/schemarunner"
	"github.com/upbound/up/internal/xpkg/workspace"
	"github.com/upbound/up/internal/yaml"
	projectv1alpha1 "github.com/upbound/up/pkg/apis/project/v1alpha1"
)

func (c *renderCmd) Help() string {
	return `
The 'render' command shows you what composed resources Crossplane would create by
printing them to stdout. It also prints any changes that would be made to the
status of the XR. It doesn't talk to Crossplane. Instead it runs the Composition
Function pipeline specified by the Composition locally, and uses that to render
the XR.

Use the standard DOCKER_HOST, DOCKER_API_VERSION, DOCKER_CERT_PATH, and
DOCKER_TLS_VERIFY environment variables to configure how this command connects
to the Docker daemon.

Examples:

  # Simulate creating a new XR.
  composition render composition.yaml xr.yaml

  # Simulate updating an XR that already exists.
  composition render composition.yaml xr.yaml \
    --observed-resources=existing-observed-resources.yaml

  # Pass context values to the Function pipeline.
  composition render composition.yaml xr.yaml \
    --context-values=apiextensions.crossplane.io/environment='{"key": "value"}'

  # Pass extra resources Functions in the pipeline can request.
  composition render composition.yaml xr.yaml \
	--extra-resources=extra-resources.yaml

  # Pass credentials to Functions in the pipeline that need them.
  composition render composition.yaml xr.yaml \
	--function-credentials=credentials.yaml
`
}

type renderCmd struct {
	Composition       string `arg:"" help:"A YAML file specifying the Composition to use to render the Composite Resource (XR)." type:"existingfile"`
	CompositeResource string `arg:"" help:"A YAML file specifying the Composite Resource (XR) to render."                        type:"existingfile"`

	XRD                    string            `help:"A YAML file specifying the CompositeResourceDefinition (XRD) to validate the XR against."                                                  optional:""        placeholder:"PATH" type:"existingfile"`
	ContextFiles           map[string]string `help:"Comma-separated context key-value pairs to pass to the Function pipeline. Values must be files containing JSON."                           mapsep:""`
	ContextValues          map[string]string `help:"Comma-separated context key-value pairs to pass to the Function pipeline. Values must be JSON. Keys take precedence over --context-files." mapsep:""`
	IncludeFunctionResults bool              `help:"Include informational and warning messages from Functions in the rendered output as resources of kind: Result."                            short:"r"`
	IncludeFullXR          bool              `help:"Include a direct copy of the input XR's spec and metadata fields in the rendered output."                                                  short:"x"`
	ObservedResources      string            `help:"A YAML file or directory of YAML files specifying the observed state of composed resources."                                               placeholder:"PATH" short:"o"          type:"path"`
	ExtraResources         string            `help:"A YAML file or directory of YAML files specifying extra resources to pass to the Function pipeline."                                       placeholder:"PATH" short:"e"          type:"path"`
	IncludeContext         bool              `help:"Include the context in the rendered output as a resource of kind: Context."                                                                short:"c"`
	FunctionCredentials    string            `help:"A YAML file or directory of YAML files specifying credentials to use for Functions to render the XR."                                      placeholder:"PATH" type:"path"`

	Timeout        time.Duration `default:"1m" help:"How long to run before timing out."`
	MaxConcurrency uint          `default:"8"  env:"UP_MAX_CONCURRENCY"                  help:"Maximum number of functions to build at once."`

	ProjectFile   string `default:"upbound.yaml"      help:"Path to project definition file."         short:"f"`
	CacheDir      string `default:"~/.up/cache/"      env:"CACHE_DIR"                                 help:"Directory used for caching dependency images." short:"d" type:"path"`
	NoBuildCache  bool   `default:"false"             help:"Don't cache image layers while building."`
	BuildCacheDir string `default:"~/.up/build-cache" help:"Path to the build cache directory."       type:"path"`

	projFS afero.Fs
	proj   *projectv1alpha1.Project

	functionIdentifier functions.Identifier
	schemaRunner       schemarunner.SchemaRunner
	concurrency        uint

	compositionRel         string
	compositeResourceRel   string
	observedResourcesRel   string
	extraResourcesRel      string
	functionCredentialsRel string
	xrdRel                 string

	m  *manager.Manager
	r  manager.ImageResolver
	ws *workspace.Workspace

	quiet config.QuietFlag
}

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *renderCmd) AfterApply(kongCtx *kong.Context, p pterm.TextPrinter, quiet config.QuietFlag) error {
	c.concurrency = max(1, c.MaxConcurrency)

	kongCtx.Bind(pterm.DefaultBulletList.WithWriter(kongCtx.Stdout))
	ctx := context.Background()

	// Read the project file.
	projFilePath, err := filepath.Abs(c.ProjectFile)
	if err != nil {
		return err
	}
	// The location of the project file defines the root of the project.
	projDirPath := filepath.Dir(projFilePath)
	c.projFS = afero.NewBasePathFs(afero.NewOsFs(), projDirPath)

	// parse the project and apply defaults.
	proj, err := project.Parse(c.projFS, c.ProjectFile)
	if err != nil {
		return err
	}
	proj.Default()

	c.proj = proj

	fs := afero.NewOsFs()

	pathMappings := []struct {
		pathField string
		relField  *string
	}{
		{c.Composition, &c.compositionRel},
		{c.CompositeResource, &c.compositeResourceRel},
		{c.FunctionCredentials, &c.functionCredentialsRel},
		{c.ObservedResources, &c.observedResourcesRel},
		{c.ExtraResources, &c.extraResourcesRel},
		{c.XRD, &c.xrdRel},
	}

	for _, mapping := range pathMappings {
		if err := c.setRelativePath(mapping.pathField, mapping.relField); err != nil {
			return err
		}
	}

	cache, err := xcache.NewLocal(c.CacheDir, xcache.WithFS(fs))
	if err != nil {
		return err
	}

	r := image.NewResolver()

	m, err := manager.New(
		manager.WithCache(cache),
		manager.WithResolver(r),
	)
	if err != nil {
		return err
	}

	c.m = m
	c.r = r

	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	ws, err := workspace.New(wd,
		workspace.WithFS(fs),
		workspace.WithPrinter(p),
		workspace.WithPermissiveParser(),
	)
	if err != nil {
		return err
	}
	c.ws = ws

	if err := ws.Parse(ctx); err != nil {
		return err
	}

	c.functionIdentifier = functions.DefaultIdentifier
	c.schemaRunner = schemarunner.RealSchemaRunner{}

	// workaround interfaces not being bindable ref: https://github.com/alecthomas/kong/issues/48
	kongCtx.BindTo(ctx, (*context.Context)(nil))

	logger := logging.NewLogrLogger(zap.New(zap.UseDevMode(false)))
	kongCtx.BindTo(logger, (*logging.Logger)(nil))

	c.quiet = quiet
	return nil
}

func (c *renderCmd) Run(log logging.Logger) error { //nolint:gocognit // same than upstream
	pterm.EnableStyling()
	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	xr, err := xprender.LoadCompositeResource(c.projFS, c.compositeResourceRel)
	if err != nil {
		return errors.Wrapf(err, "cannot load composite resource from %q", c.compositeResourceRel)
	}

	comp, err := xprender.LoadComposition(c.projFS, c.compositionRel)
	if err != nil {
		return errors.Wrapf(err, "cannot load Composition from %q", c.compositionRel)
	}

	expected := comp.Spec.CompositeTypeRef
	actual := xr.GetReference()
	if expected.APIVersion != actual.APIVersion || expected.Kind != actual.Kind {
		return errors.Errorf(
			"CompositeResource %s.%s does not match Composition compositeTypeRef %s.%s",
			actual.Kind, actual.APIVersion,
			expected.Kind, expected.APIVersion,
		)
	}

	if c.XRD != "" {
		xrd, err := loadXRD(c.projFS, c.xrdRel)
		if err != nil {
			return errors.Wrapf(err, "cannot load XRD from %q", c.xrdRel)
		}
		crd, err := xcrd.ForCompositeResource(xrd)
		if err != nil {
			return errors.Wrapf(err, "cannot derive composite CRD from XRD %q", xrd.GetName())
		}
		if err := icrd.DefaultValues(xr.UnstructuredContent(), *crd); err != nil {
			return errors.Wrapf(err, "cannot default values for XR %q", xr.GetName())
		}
	}

	fcreds := []corev1.Secret{}
	if c.FunctionCredentials != "" {
		fcreds, err = xprender.LoadCredentials(c.projFS, c.functionCredentialsRel)
		if err != nil {
			return errors.Wrapf(err, "cannot load secrets from %q", c.functionCredentialsRel)
		}
	}

	ors := []composed.Unstructured{}
	if c.ObservedResources != "" {
		ors, err = xprender.LoadObservedResources(c.projFS, c.observedResourcesRel)
		if err != nil {
			return errors.Wrapf(err, "cannot load observed composed resources from %q", c.observedResourcesRel)
		}
	}

	ers := []unstructured.Unstructured{}
	if c.ExtraResources != "" {
		ers, err = xprender.LoadExtraResources(c.projFS, c.extraResourcesRel)
		if err != nil {
			return errors.Wrapf(err, "cannot load extra resources from %q", c.extraResourcesRel)
		}
	}

	fctx := map[string][]byte{}
	for k, filename := range c.ContextFiles {
		v, err := afero.ReadFile(c.projFS, filename)
		if err != nil {
			return errors.Wrapf(err, "cannot read context value for key %q", k)
		}
		fctx[k] = v
	}
	for k, v := range c.ContextValues {
		fctx[k] = []byte(v)
	}

	// build embedded functions
	b := project.NewBuilder(
		project.BuildWithMaxConcurrency(c.concurrency),
		project.BuildWithFunctionIdentifier(c.functionIdentifier),
		project.BuildWithSchemaRunner(c.schemaRunner),
	)

	// ToDo(haarchri): consider building only functions which configured in composition
	var imgMap project.ImageTagMap
	err = async.WrapWithSuccessSpinners(func(ch async.EventChannel) error {
		var err error
		imgMap, err = b.Build(ctx, c.proj, c.projFS,
			project.BuildWithEventChannel(ch, c.quiet),
			project.BuildWithImageLabels(common.ImageLabels(c)),
			project.BuildWithDependencyManager(c.m))
		return err
	})
	if err != nil {
		return err
	}

	if !c.NoBuildCache {
		// Create a layer cache so that if we're building on top of base images we
		// only pull their layers once. Note we do this here rather than in the
		// builder because pulling layers is deferred to where we use them, which is
		// here.
		cch := cache.NewValidatingCache(v1cache.NewFilesystemCache(c.BuildCacheDir))
		for tag, img := range imgMap {
			imgMap[tag] = v1cache.Image(img, cch)
		}
	}

	var efns []pkgv1.Function
	err = upterm.WrapWithSuccessSpinner(
		"Pushing embedded functions to local daemon",
		upterm.CheckmarkSuccessSpinner,
		func() error {
			// push embedded functions to daemon
			lefns, err := embeddedFunctionsToDaemon(imgMap)
			if err != nil {
				return errors.Wrap(err, "unable to push to local docker daemon")
			}
			efns = lefns
			return nil
		},
		c.quiet,
	)
	if err != nil {
		return err
	}
	// load functions from project upbound.yaml dependsOn
	fns, err := c.loadFunctions(ctx, c.proj)
	if err != nil {
		return errors.Wrapf(err, "cannot load functions from project")
	}

	// collect functions from project upbound.yaml and embedded functions
	fns = append(fns, efns...)

	var out xprender.Outputs
	err = upterm.WrapWithSuccessSpinner(
		"Rendering",
		upterm.CheckmarkSuccessSpinner,
		func() error {
			lout, err := xprender.Render(ctx, log, xprender.Inputs{
				CompositeResource:   xr,
				Composition:         comp,
				Functions:           fns,
				FunctionCredentials: fcreds,
				ObservedResources:   ors,
				ExtraResources:      ers,
				Context:             fctx,
			})
			if err != nil {
				return errors.Wrap(err, "cannot render composite resource")
			}
			out = lout
			return nil
		},
		c.quiet,
	)
	if err != nil {
		return err
	}

	s := json.NewSerializerWithOptions(json.DefaultMetaFactory, nil, nil, json.SerializerOptions{Yaml: true})

	if c.IncludeFullXR {
		xrSpec, err := fieldpath.Pave(xr.Object).GetValue("spec")
		if err != nil {
			return errors.Wrapf(err, "cannot get composite resource spec")
		}

		if err := fieldpath.Pave(out.CompositeResource.Object).SetValue("spec", xrSpec); err != nil {
			return errors.Wrapf(err, "cannot set composite resource spec")
		}

		xrMeta, err := fieldpath.Pave(xr.Object).GetValue("metadata")
		if err != nil {
			return errors.Wrapf(err, "cannot get composite resource metadata")
		}

		if err := fieldpath.Pave(out.CompositeResource.Object).SetValue("metadata", xrMeta); err != nil {
			return errors.Wrapf(err, "cannot set composite resource metadata")
		}
	}

	// when using p.Println we have 2 new-lines when using with kongCtx.Stdout
	if _, err := fmt.Fprintln(os.Stdout, "---"); err != nil {
		return errors.Wrap(err, "failed to write to standard output")
	}
	if err := s.Encode(out.CompositeResource, os.Stdout); err != nil {
		return errors.Wrapf(err, "cannot marshal composite resource %q to YAML", xr.GetName())
	}

	for i := range out.ComposedResources {
		if _, err := fmt.Fprintln(os.Stdout, "---"); err != nil {
			return errors.Wrap(err, "failed to write to standard output")
		}
		if err := s.Encode(&out.ComposedResources[i], os.Stdout); err != nil {
			return errors.Wrapf(err, "cannot marshal composed resource to YAML")
		}
	}

	if c.IncludeFunctionResults {
		for i := range out.Results {
			if _, err := fmt.Fprintln(os.Stdout, "---"); err != nil {
				return errors.Wrap(err, "failed to write to standard output")
			}
			if err := s.Encode(&out.Results[i], os.Stdout); err != nil {
				return errors.Wrap(err, "cannot marshal result to YAML")
			}
		}
	}

	if c.IncludeContext {
		if _, err := fmt.Fprintln(os.Stdout, "---"); err != nil {
			return errors.Wrap(err, "failed to write to standard output")
		}
		if err := s.Encode(out.Context, os.Stdout); err != nil {
			return errors.Wrap(err, "cannot marshal context to YAML")
		}
	}

	return nil
}

// loadFunctions from a stream of YAML manifests.
func (c *renderCmd) loadFunctions(ctx context.Context, proj *projectv1alpha1.Project) ([]pkgv1.Function, error) {
	functions := make([]pkgv1.Function, 0, len(proj.Spec.DependsOn))

	for _, dep := range proj.Spec.DependsOn {
		if dep.Function == nil {
			continue
		}

		// convert fn to dependency
		convertedDep, ok := manager.ConvertToV1beta1(dep)
		if !ok {
			return nil, errors.Errorf("failed to convert dependency in %s", *dep.Function)
		}

		// resolve tag for fn
		version, err := c.r.ResolveTag(ctx, convertedDep)
		if err != nil {
			return nil, errors.Wrapf(err, "failed resolve tag")
		}

		// get metadata.name for fn pkg manifest
		functionName, err := name.ParseReference(*dep.Function)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse function name reference for %s", *dep.Function)
		}

		// build fn pkg manifest
		f := pkgv1.Function{
			ObjectMeta: metav1.ObjectMeta{
				Name: xpkg.ToDNSLabel(functionName.Context().RepositoryStr()),
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

// Helper function to calculate the relative path and handle errors.
func (c *renderCmd) setRelativePath(fieldValue string, relativePath *string) error {
	if fieldValue != "" {
		relPath, err := filepath.Rel(afero.FullBaseFsPath(c.projFS.(*afero.BasePathFs), "."), fieldValue)
		if err != nil {
			return errors.Wrap(err, "failed to make file path relative to project filesystem")
		}
		*relativePath = relPath
	}
	return nil
}

func loadXRD(fs afero.Fs, file string) (*apiextensionsv1.CompositeResourceDefinition, error) {
	y, err := afero.ReadFile(fs, file)
	if err != nil {
		return nil, errors.Wrap(err, "cannot read XRD file")
	}
	xrd := &apiextensionsv1.CompositeResourceDefinition{}
	return xrd, errors.Wrap(yaml.Unmarshal(y, xrd), "cannot unmarshal XRD YAML")
}
