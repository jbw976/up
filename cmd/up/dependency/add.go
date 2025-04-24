// Copyright 2025 Upbound Inc.
// All rights reserved

package dependency

import (
	"context"
	"io"
	"path/filepath"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"

	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/internal/xpkg"
	"github.com/upbound/up/internal/xpkg/dep"
	"github.com/upbound/up/internal/xpkg/dep/cache"
	"github.com/upbound/up/internal/xpkg/dep/manager"
	"github.com/upbound/up/internal/xpkg/dep/resolver/image"
	"github.com/upbound/up/internal/xpkg/workspace"
)

func (c *addCmd) Help() string {
	return `
The 'add' command retrieves a Crossplane package (provider, configuration, or function) from a specified registry with an optional version tag and adds it to a project as a dependency.

Examples:
    dependency add xpkg.upbound.io/upbound/provider-aws-eks
        Retrieves the provider, adds all CRDs to the cache folder,
		and places all language schemas in the repository root .up/ folder.
		Uses the latest available package.

    dependency add 'xpkg.upbound.io/upbound/platform-ref-aws:>v1.1.0'
        Retrieves the configuration, adds all XRDs to the cache folder,
		and places all language schemas in the repository root .up/ folder.
		Uses a package version greater than v1.1.0.

    dependency add 'xpkg.upbound.io/crossplane-contrib/function-kcl:>=v0.10.8'
        Retrieves the function, adds all CRDs to the cache folder,
		and places all language schemas in the repository root .up/ folder.
		Uses a package version v0.10.8 or newer, if available.
`
}

// addCmd manages crossplane dependencies.
type addCmd struct {
	m        *manager.Manager
	ws       *workspace.Workspace
	modelsFS afero.Fs

	Package     string `arg:""                 help:"Package to be added."`
	ProjectFile string `default:"upbound.yaml" help:"Path to project definition file." short:"f"`

	// TODO(@tnthornton) remove cacheDir flag. Having a user supplied flag
	// can result in broken behavior between xpls and dep. CacheDir should
	// only be supplied by the Config.
	CacheDir string `default:"~/.up/cache/" env:"CACHE_DIR" help:"Directory used for caching package images." type:"path"`
}

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *addCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	kongCtx.Bind(pterm.DefaultBulletList.WithWriter(kongCtx.Stdout))
	ctx := context.Background()

	// Read the project file.
	projFilePath, err := filepath.Abs(c.ProjectFile)
	if err != nil {
		return err
	}
	// The location of the project file defines the root of the project.
	projDirPath := filepath.Dir(projFilePath)
	projFS := afero.NewBasePathFs(afero.NewOsFs(), projDirPath)
	c.modelsFS = afero.NewBasePathFs(afero.NewOsFs(), filepath.Join(projDirPath, ".up"))

	prj, err := project.Parse(projFS, c.ProjectFile)
	if err != nil {
		return errors.New("this is not a project directory")
	}

	fs := afero.NewOsFs()

	cache, err := cache.NewLocal(c.CacheDir, cache.WithFS(fs))
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
		manager.WithCache(cache),
		manager.WithResolver(r),
	)
	if err != nil {
		return err
	}

	c.m = m

	ws, err := workspace.New("/",
		workspace.WithFS(projFS),
		// The user doesn't care about workspace warnings.
		workspace.WithPrinter(&pterm.BasicTextPrinter{Writer: io.Discard}),
		workspace.WithPermissiveParser(),
	)
	if err != nil {
		return err
	}
	c.ws = ws

	if err := ws.Parse(ctx); err != nil {
		return err
	}

	// workaround interfaces not being bindable ref: https://github.com/alecthomas/kong/issues/48
	kongCtx.BindTo(ctx, (*context.Context)(nil))
	return nil
}

// Run executes the dep command.
func (c *addCmd) Run(ctx context.Context, _ pterm.TextPrinter) error {
	// ToDo(haarchri): rebase
	pterm.EnableStyling()

	_, err := xpkg.ValidDep(c.Package)
	if err != nil {
		return err
	}

	d := dep.New(c.Package)

	var ud v1beta1.Dependency
	if err = upterm.WrapWithSuccessSpinner(
		"Updating cache dependencies...",
		upterm.CheckmarkSuccessSpinner,
		func() error {
			ud, _, err = c.m.AddAll(ctx, d)
			if err != nil {
				return errors.Wrapf(err, "in %s", c.Package)
			}
			return nil
		},
		false,
	); err != nil {
		return err
	}
	pterm.Success.Printfln("%s:%s added to cache", ud.Package, ud.Constraints)
	meta := c.ws.View().Meta()

	if meta != nil {
		if err = upterm.WrapWithSuccessSpinner(
			"Updating project dependencies...",
			upterm.CheckmarkSuccessSpinner,
			func() error {
				// Metadata file exists in the workspace, upsert the new dependency
				// use the user-specified constraints if provided; otherwise, use latest
				if d.Constraints != "" {
					ud.Constraints = d.Constraints
				}
				if err := meta.Upsert(ud); err != nil {
					return err
				}

				if err := c.ws.Write(meta); err != nil {
					return err
				}
				return nil
			},
			false,
		); err != nil {
			return err
		}
	}
	pterm.Success.Printfln("%s:%s added to project dependency", ud.Package, ud.Constraints)
	return nil
}
