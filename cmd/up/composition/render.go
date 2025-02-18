// Copyright 2025 Upbound Inc.
// All rights reserved

// Package composition contains functions for local composition rendering
package composition

import (
	"context"
	"path/filepath"
	"time"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	"github.com/spf13/afero"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	v1 "github.com/crossplane/crossplane/apis/pkg/v1"

	"github.com/upbound/up/internal/async"
	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/render"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
	xcache "github.com/upbound/up/internal/xpkg/dep/cache"
	"github.com/upbound/up/internal/xpkg/dep/manager"
	"github.com/upbound/up/internal/xpkg/dep/resolver/image"
	"github.com/upbound/up/internal/xpkg/functions"
	"github.com/upbound/up/internal/xpkg/schemarunner"
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

	Flags upbound.Flags `embed:""`

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

	m *manager.Manager
	r manager.ImageResolver

	quiet        config.QuietFlag
	asyncWrapper async.WrapperFunc
}

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *renderCmd) AfterApply(kongCtx *kong.Context, quiet config.QuietFlag) error {
	c.concurrency = max(1, c.MaxConcurrency)

	kongCtx.Bind(pterm.DefaultBulletList.WithWriter(kongCtx.Stdout))
	ctx := context.Background()

	upCtx, err := upbound.NewFromFlags(c.Flags)
	if err != nil {
		return err
	}

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

	r := image.NewResolver(
		image.WithFetcher(
			image.NewLocalFetcher(
				image.WithKeychain(upCtx.RegistryKeychain()),
			),
		),
	)

	m, err := manager.New(
		manager.WithCache(cache),
		manager.WithResolver(r),
	)
	if err != nil {
		return err
	}

	c.m = m
	c.r = r

	c.functionIdentifier = functions.DefaultIdentifier
	c.schemaRunner = schemarunner.RealSchemaRunner{}

	// workaround interfaces not being bindable ref: https://github.com/alecthomas/kong/issues/48
	kongCtx.BindTo(ctx, (*context.Context)(nil))

	logger := logging.NewLogrLogger(zap.New(zap.UseDevMode(false)))
	kongCtx.BindTo(logger, (*logging.Logger)(nil))

	c.quiet = quiet
	c.asyncWrapper = async.WrapWithSuccessSpinners
	if quiet {
		c.asyncWrapper = async.IgnoreEvents
	}

	return nil
}

func (c *renderCmd) Run(log logging.Logger) error {
	pterm.EnableStyling()

	renderCtx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	var efns []v1.Function
	err := c.asyncWrapper(func(ch async.EventChannel) error {
		functionOptions := render.FunctionOptions{
			Project:            c.proj,
			ProjFS:             c.projFS,
			Concurrency:        c.concurrency,
			NoBuildCache:       c.NoBuildCache,
			BuildCacheDir:      c.BuildCacheDir,
			DependecyManager:   c.m,
			FunctionIdentifier: c.functionIdentifier,
			SchemaRunner:       c.schemaRunner,
			EventChannel:       ch,
		}

		fns, err := render.BuildEmbeddedFunctionsLocalDaemon(renderCtx, functionOptions)
		if err != nil {
			return errors.Wrap(err, "unable to build embedded functions")
		}
		efns = fns

		return nil
	})
	if err != nil {
		return err
	}

	options := render.Options{
		Project:                c.proj,
		ProjFS:                 c.projFS,
		IncludeFullXR:          c.IncludeFullXR,
		IncludeFunctionResults: c.IncludeFunctionResults,
		IncludeContext:         c.IncludeContext,
		CompositeResource:      c.compositeResourceRel,
		Composition:            c.compositionRel,
		XRD:                    c.xrdRel,
		FunctionCredentials:    c.functionCredentialsRel,
		ObservedResources:      c.observedResourcesRel,
		ExtraResources:         c.extraResourcesRel,
		ContextFiles:           c.ContextFiles,
		ContextValues:          c.ContextValues,
		Concurrency:            c.concurrency,
		ImageResolver:          c.r,
	}

	return upterm.WrapWithSuccessSpinner("Rendering", upterm.CheckmarkSuccessSpinner, func() error {
		output, err := render.Render(renderCtx, log, efns, options)
		if err != nil {
			return errors.Wrap(err, "unable to render function")
		}
		pterm.Print(output)
		return nil
	},
		c.quiet,
	)
}

// Helper function to calculate the relative path and handle errors.
func (c *renderCmd) setRelativePath(fieldValue string, relativePath *string) error {
	if fieldValue != "" {
		relPath, err := filepath.Rel(afero.FullBaseFsPath(c.projFS.(*afero.BasePathFs), "."), fieldValue) //nolint:forcetypeassert // We know the type of projFS from above.
		if err != nil {
			return errors.Wrap(err, "failed to make file path relative to project filesystem")
		}
		*relativePath = relPath
	}
	return nil
}
