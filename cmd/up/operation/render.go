// Copyright 2025 Upbound Inc.
// All rights reserved

// Package operation contains functions for local operation rendering
package operation

import (
	"context"
	"path/filepath"
	"time"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	v1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"

	"github.com/upbound/up/internal/async"
	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/render"
	"github.com/upbound/up/internal/render/operations"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/internal/xpkg/dep/manager"
	"github.com/upbound/up/internal/xpkg/dep/resolver/image"
	"github.com/upbound/up/internal/xpkg/functions"
	projectv2alpha1 "github.com/upbound/up/pkg/apis/project/v2alpha1"

	_ "embed"
)

//go:embed help/render.md
var renderHelp string

func (c *renderCmd) Help() string {
	return renderHelp
}

type renderCmd struct {
	Operation string `arg:"" help:"A YAML file specifying the Operation to render." type:"existingfile"`

	RequiredResources      string            `help:"A YAML file or directory of YAML files specifying required resources that functions can request."                                          placeholder:"PATH"      short:"r"   type:"path"`
	ContextFiles           map[string]string `help:"Comma-separated context key-value pairs to pass to the Function pipeline. Values must be files containing JSON."                           mapsep:""`
	ContextValues          map[string]string `help:"Comma-separated context key-value pairs to pass to the Function pipeline. Values must be JSON. Keys take precedence over --context-files." mapsep:""`
	IncludeFunctionResults bool              `help:"Include informational and warning messages from Functions in the rendered output as resources of kind: Result."                            short:"f"`
	IncludeFullOperation   bool              `help:"Include the full Operation with original spec and metadata in the rendered output."                                                        short:"o"`
	IncludeContext         bool              `help:"Include the context in the rendered output as a resource of kind: Context."                                                                short:"c"`
	FunctionCredentials    string            `help:"A YAML file or directory of YAML files specifying credentials to use for Functions to render the Operation."                               placeholder:"PATH"      type:"path"`
	FunctionAnnotations    []string          `help:"Override function annotations for all functions. Can be repeated."                                                                         placeholder:"KEY=VALUE"`

	Timeout        time.Duration `default:"1m" help:"How long to run before timing out."`
	MaxConcurrency uint          `default:"8"  env:"UP_MAX_CONCURRENCY"                  help:"Maximum number of functions to build at once."`

	ProjectFile   string `default:"upbound.yaml"      help:"Path to project definition file."         short:"p"`
	CacheDir      string `default:"~/.up/cache/"      env:"CACHE_DIR"                                 help:"Directory used for caching dependency images." type:"path"`
	NoBuildCache  bool   `default:"false"             help:"Don't cache image layers while building."`
	BuildCacheDir string `default:"~/.up/build-cache" help:"Path to the build cache directory."       type:"path"`

	projFS afero.Fs
	proj   *projectv2alpha1.Project

	functionIdentifier functions.Identifier
	concurrency        uint

	operationRel           string
	requiredResourcesRel   string
	functionCredentialsRel string

	m *project.DependencyManager
	r manager.ImageResolver

	quiet        config.QuietFlag
	asyncWrapper async.WrapperFunc
}

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *renderCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context, printer upterm.ObjectPrinter) error {
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
	proj, err := project.Parse(c.projFS, filepath.Base(c.ProjectFile))
	if err != nil {
		return err
	}
	proj.Default()

	c.proj = proj

	pathMappings := []struct {
		pathField string
		relField  *string
	}{
		{c.Operation, &c.operationRel},
		{c.RequiredResources, &c.requiredResourcesRel},
		{c.FunctionCredentials, &c.functionCredentialsRel},
	}

	for _, mapping := range pathMappings {
		if err := c.setRelativePath(mapping.pathField, mapping.relField); err != nil {
			return err
		}
	}

	r := image.NewResolver(
		image.WithImageConfig(proj.Spec.ImageConfig),
		image.WithFetcher(
			image.NewLocalFetcher(
				image.WithKeychain(upCtx.RegistryKeychain()),
			),
		),
	)

	cchFS := afero.NewBasePathFs(afero.NewOsFs(), c.CacheDir)
	m, err := project.NewDependencyManager(upCtx, proj, c.projFS,
		project.WithCacheFS(cchFS),
	)
	if err != nil {
		return err
	}

	c.m = m
	c.r = r

	c.functionIdentifier = functions.DefaultIdentifier
	// workaround interfaces not being bindable ref: https://github.com/alecthomas/kong/issues/48
	kongCtx.BindTo(ctx, (*context.Context)(nil))

	logger := logging.NewNopLogger()
	kongCtx.BindTo(logger, (*logging.Logger)(nil))

	c.quiet = printer.Quiet
	switch {
	case bool(printer.Quiet):
		c.asyncWrapper = async.IgnoreEvents
	case printer.Pretty:
		c.asyncWrapper = async.WrapWithSuccessSpinnersPretty
	default:
		c.asyncWrapper = async.WrapWithSuccessSpinnersNonPretty
	}

	return nil
}

func (c *renderCmd) Run(ctx context.Context, upCtx *upbound.Context, log logging.Logger, printer upterm.ObjectPrinter) error {
	var efns []v1.Function
	err := c.asyncWrapper(func(ch async.EventChannel) error {
		functionOptions := render.FunctionOptions{
			Project:            c.proj,
			ProjFS:             c.projFS,
			Concurrency:        c.concurrency,
			NoBuildCache:       c.NoBuildCache,
			BuildCacheDir:      c.BuildCacheDir,
			DependencyManager:  c.m,
			FunctionIdentifier: c.functionIdentifier,
			EventChannel:       ch,
		}

		fns, err := render.BuildEmbeddedFunctionsLocalDaemon(ctx, upCtx, functionOptions)
		if err != nil {
			return errors.Wrap(err, "unable to build embedded functions")
		}
		efns = fns

		return nil
	})
	if err != nil {
		return err
	}

	options := operations.Options{
		Project:                c.proj,
		ProjFS:                 c.projFS,
		IncludeFullOperation:   c.IncludeFullOperation,
		IncludeFunctionResults: c.IncludeFunctionResults,
		IncludeContext:         c.IncludeContext,
		Operation:              c.operationRel,
		FunctionCredentials:    c.functionCredentialsRel,
		RequiredResources:      c.requiredResourcesRel,
		ContextFiles:           c.ContextFiles,
		ContextValues:          c.ContextValues,
		Concurrency:            c.concurrency,
		ImageResolver:          c.r,
		FunctionAnnotations:    c.FunctionAnnotations,
	}

	renderCtx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()

	var output string
	if err := upterm.WrapWithSuccessSpinner("Rendering", func() error {
		output, err = operations.Render(renderCtx, log, efns, options)
		if err != nil {
			return errors.Wrap(err, "unable to render operation")
		}
		return nil
	}, printer); err != nil {
		return err
	}

	pterm.Print(output)
	return nil
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
