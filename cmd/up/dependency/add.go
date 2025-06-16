// Copyright 2025 Upbound Inc.
// All rights reserved

package dependency

import (
	"context"
	"path/filepath"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	"github.com/spf13/afero"
	"k8s.io/utils/ptr"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"

	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/pkg/apis/project/v1alpha1"
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
	m        *project.DependencyManager
	modelsFS afero.Fs
	projFS   afero.Fs
	proj     *v1alpha1.Project

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
	c.projFS = afero.NewBasePathFs(afero.NewOsFs(), projDirPath)
	c.modelsFS = afero.NewBasePathFs(c.projFS, ".up")

	prj, err := project.Parse(c.projFS, c.ProjectFile)
	if err != nil {
		return errors.New("this is not a project directory")
	}
	c.proj = prj
	// We don't want to call Default() here since we would end up writing out
	// defaults to the project file. Just make sure spec is non-nil.
	if c.proj.Spec == nil {
		c.proj.Spec = &v1alpha1.ProjectSpec{}
	}

	m, err := project.NewDependencyManager(upCtx, c.proj, c.projFS,
		project.WithCacheFS(afero.NewBasePathFs(afero.NewOsFs(), c.CacheDir)),
	)
	if err != nil {
		return err
	}

	c.m = m

	// workaround interfaces not being bindable ref: https://github.com/alecthomas/kong/issues/48
	kongCtx.BindTo(ctx, (*context.Context)(nil))
	return nil
}

// Run executes the dep command.
func (c *addCmd) Run(ctx context.Context, printer upterm.ObjectPrinter) error {
	var d pkgmetav1.Dependency
	if err := upterm.WrapWithSuccessSpinner(
		"Adding dependency...",
		upterm.CheckmarkSuccessSpinner,
		func() error {
			var err error
			d, err = c.m.AddByRef(ctx, c.Package)
			return err
		},
		printer,
	); err != nil {
		return err
	}
	pterm.Success.Printfln("%s:%s added", ptr.Deref(d.Package, ""), d.Version)

	return nil
}
