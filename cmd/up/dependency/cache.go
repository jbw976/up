// Copyright 2025 Upbound Inc.
// All rights reserved

package dependency

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/alecthomas/kong"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/internal/xpkg/dep/cache"
	"github.com/upbound/up/pkg/apis/project/v2alpha1"

	_ "embed"
)

// updateCacheCmd updates the cache.
type updateCacheCmd struct {
	m      *project.DependencyManager
	projFS afero.Fs
	proj   *v2alpha1.Project

	ProjectFile string `default:"upbound.yaml" help:"Path to project definition file." short:"f"`
	// TODO(@tnthornton) remove cacheDir flag. Having a user supplied flag
	// can result in broken behavior between xpls and dep. CacheDir should
	// only be supplied by the Config.
	CacheDir string `default:"~/.up/cache/" env:"CACHE_DIR" help:"Directory used for caching package images." type:"path"`
}

//go:embed help/update-cache.md
var updateCacheHelp string

// Help returns help.
func (c *updateCacheCmd) Help() string {
	return updateCacheHelp
}

func (c *updateCacheCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	ctx := context.Background()

	// Read the project file.
	projFilePath, err := filepath.Abs(c.ProjectFile)
	if err != nil {
		return err
	}
	// The location of the project file defines the root of the project.
	projDirPath := filepath.Dir(projFilePath)
	c.projFS = afero.NewBasePathFs(afero.NewOsFs(), projDirPath)

	prj, err := project.Parse(c.projFS, c.ProjectFile)
	if err != nil {
		return errors.New("this is not a project directory")
	}
	prj.Default()
	c.proj = prj

	cchFS := afero.NewBasePathFs(afero.NewOsFs(), c.CacheDir)
	m, err := project.NewDependencyManager(upCtx, c.proj, c.projFS,
		project.WithCacheFS(cchFS),
	)
	if err != nil {
		return err
	}

	c.m = m

	kongCtx.BindTo(ctx, (*context.Context)(nil))
	return nil
}

func (c *updateCacheCmd) Run(ctx context.Context, printer upterm.Printer) error {
	if len(c.proj.Spec.DependsOn) > 0 {
		if err := printer.WrapWithSuccessSpinner(
			fmt.Sprintf("Updating %d dependencies...", len(c.proj.Spec.DependsOn)),
			func() error {
				return c.m.AddAll(ctx, c.proj.Spec.DependsOn...)
			},
		); err != nil {
			return err
		}

		printer.PrintSuccess("Dependencies updated:")
		for _, d := range c.proj.Spec.DependsOn {
			pkg, err := c.m.GetParsedPackage(ctx, d)
			if err != nil {
				return err
			}
			printer.PrintSuccess(fmt.Sprintf("- %s (%s)", pkg.Name(), pkg.Version()))
		}
	}

	if len(c.proj.Spec.APIDependencies) > 0 {
		if err := printer.WrapWithSuccessSpinner(
			fmt.Sprintf("Updating %d api-dependencies...", len(c.proj.Spec.APIDependencies)),
			func() error {
				return c.m.AddAllAPIDependencies(ctx, c.proj.Spec.APIDependencies)
			},
		); err != nil {
			return err
		}

		processedAPIDeps, err := c.m.GetProcessedAPIDependencies(ctx, c.proj.Spec.APIDependencies)
		if err != nil {
			return err
		}

		printer.PrintSuccess("API dependencies updated:")
		for _, dep := range processedAPIDeps {
			printer.PrintSuccess(fmt.Sprintf("- %s (%s)", dep.Source, dep.Type))
		}
	}

	return nil
}

// cleanCacheCmd updates the cache.
type cleanCacheCmd struct {
	c *cache.Local

	// TODO(@tnthornton) remove cacheDir flag. Having a user supplied flag
	// can result in broken behavior between xpls and dep. CacheDir should
	// only be supplied by the Config.
	CacheDir string `default:"~/.up/cache/" env:"CACHE_DIR" help:"Directory used for caching package images." type:"path"`
}

//go:embed help/clean-cache.md
var cleanCacheHelp string

// Help returns help.
func (c *cleanCacheCmd) Help() string {
	return cleanCacheHelp
}

func (c *cleanCacheCmd) AfterApply(kongCtx *kong.Context) error {
	ctx := context.Background()
	fs := afero.NewOsFs()

	cache, err := cache.NewLocal(c.CacheDir, cache.WithFS(fs))
	if err != nil {
		return err
	}

	c.c = cache

	// workaround interfaces not being bindable ref: https://github.com/alecthomas/kong/issues/48
	kongCtx.BindTo(ctx, (*context.Context)(nil))
	return nil
}

func (c *cleanCacheCmd) Run(p upterm.Printer) error {
	if err := c.c.Clean(); err != nil {
		return err
	}
	p.Println("xpkg cache cleaned")
	return nil
}
