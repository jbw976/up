// Copyright 2025 Upbound Inc.
// All rights reserved

package dependency

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"

	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/internal/xpkg/dep/cache"
	"github.com/upbound/up/internal/xpkg/dep/manager"
	"github.com/upbound/up/internal/xpkg/dep/resolver/image"
	"github.com/upbound/up/pkg/apis/project/v1alpha1"
)

// updateCacheCmd updates the cache.
type updateCacheCmd struct {
	m        *manager.Manager
	modelsFS afero.Fs
	projFS   afero.Fs
	proj     *v1alpha1.Project

	ProjectFile string `default:"upbound.yaml" help:"Path to project definition file." short:"f"`
	// TODO(@tnthornton) remove cacheDir flag. Having a user supplied flag
	// can result in broken behavior between xpls and dep. CacheDir should
	// only be supplied by the Config.
	CacheDir string `default:"~/.up/cache/" env:"CACHE_DIR" help:"Directory used for caching package images." type:"path"`
}

func (c *updateCacheCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
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
	c.modelsFS = afero.NewBasePathFs(c.projFS, ".up")

	prj, err := project.Parse(c.projFS, c.ProjectFile)
	if err != nil {
		return errors.New("this is not a project directory")
	}
	prj.Default()
	c.proj = prj

	fs := afero.NewOsFs()

	cch, err := cache.NewLocal(c.CacheDir, cache.WithFS(fs))
	if err != nil {
		return err
	}

	r := image.NewResolver(
		image.WithImageConfig(prj.Spec.ImageConfig),
		image.WithFetcher(
			image.NewLocalFetcher(
				image.WithKeychain(upCtx.RegistryKeychain()),
			),
		),
	)

	m, err := manager.New(
		manager.WithCacheModels(c.modelsFS),
		manager.WithCache(cch),
		manager.WithResolver(r),
	)
	if err != nil {
		return err
	}

	c.m = m

	kongCtx.BindTo(ctx, (*context.Context)(nil))
	return nil
}

func (c *updateCacheCmd) Run(ctx context.Context, printer upterm.ObjectPrinter) error {
	metaDeps := c.proj.Spec.DependsOn

	resolvedDeps := make([]v1beta1.Dependency, len(metaDeps))
	if err := upterm.WrapWithSuccessSpinner(
		fmt.Sprintf("Updating %d dependencies...", len(metaDeps)),
		upterm.CheckmarkSuccessSpinner,
		func() error {
			for i, d := range metaDeps {
				converted, ok := manager.ConvertToV1beta1(d)
				if !ok {
					return errors.New("failed to convert dependency")
				}
				ud, _, err := c.m.AddAll(ctx, converted)
				if err != nil {
					return err
				}
				resolvedDeps[i] = ud
			}
			return nil
		},
		printer,
	); err != nil {
		return err
	}

	if len(resolvedDeps) == 0 {
		pterm.Warning.Printfln("No dependencies specified.")
		return nil
	}

	pterm.Success.Printfln("Dependencies added to cache:")
	for _, d := range resolvedDeps {
		pterm.Success.Printfln("- %s (%s)", d.Package, d.Constraints)
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

func (c *cleanCacheCmd) AfterApply(kongCtx *kong.Context) error {
	kongCtx.Bind(pterm.DefaultBulletList.WithWriter(kongCtx.Stdout))
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

func (c *cleanCacheCmd) Run(p pterm.TextPrinter) error {
	if err := c.c.Clean(); err != nil {
		return err
	}
	p.Printfln("xpkg cache cleaned")
	return nil
}
