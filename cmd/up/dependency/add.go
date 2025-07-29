// Copyright 2025 Upbound Inc.
// All rights reserved

package dependency

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	"github.com/spf13/afero"
	"k8s.io/utils/ptr"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"

	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/pkg/apis/project/v2alpha1"
)

func (c *addCmd) Help() string {
	return `
The 'add' command retrieves a Crossplane package (provider, configuration, or function) from a specified registry with an optional version tag and adds it to a project as a dependency.

For API dependencies, use --api flag.

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

    dependency add --api k8s:v1.33.0
        Adds Kubernetes API v1.33.0 as a api-dependency,
        and provides all language schemas.

    dependency add --api https://raw.githubusercontent.com/cert-manager/cert-manager/refs/heads/master/deploy/crds/cert-manager.io_certificaterequests.yaml
        Adds a CRD from an HTTP URL as api-dependency,
        and provides all language schemas.

    dependency add --api https://github.com/crossplane/crossplane --git-ref=release-1.20 --git-path=cluster/crds
        Adds CRDs from a git repository as api-dependency,
        and provides all language schemas.
`
}

// addCmd manages crossplane dependencies.
type addCmd struct {
	Package     string `arg:""                 help:"Package to be added."`
	ProjectFile string `default:"upbound.yaml" help:"Path to project definition file." short:"f"`

	// API dependency specific flags
	API     bool   `help:"Treat the dependency as an API dependency (k8s or CRD)."`
	GitRef  string `help:"Git ref for CRD dependencies (branch, tag, or commit SHA). If provided, the CRD will be fetched from git." name:"git-ref"`
	GitPath string `help:"Path within the git repository for CRD dependencies."                                                      name:"git-path"`

	// TODO(@tnthornton) remove cacheDir flag. Having a user supplied flag
	// can result in broken behavior between xpls and dep. CacheDir should
	// only be supplied by the Config.
	CacheDir string `default:"~/.up/cache/" env:"CACHE_DIR" help:"Directory used for caching package images." type:"path"`

	m        *project.DependencyManager
	modelsFS afero.Fs
	projFS   afero.Fs
	proj     *v2alpha1.Project
	apiDep   *v2alpha1.APIDependencies
}

// parseAPIDependency parses the package argument and flags to build an API dependency structure.
func (c *addCmd) parseAPIDependency() error {
	// Parse the package argument to determine the API dependency type
	if version, found := strings.CutPrefix(c.Package, "k8s:"); found {
		// k8s dependency - format: k8s:vX.Y.Z
		if version == "" {
			return errors.New("k8s version is required (e.g., k8s:v1.33.0)")
		}
		c.apiDep = &v2alpha1.APIDependencies{
			Type: v2alpha1.APIDependencyTypeK8s,
			K8s: &v2alpha1.APIK8sReference{
				Version: version,
			},
		}
		return nil
	}

	// CRD dependency
	c.apiDep = &v2alpha1.APIDependencies{
		Type: v2alpha1.APIDependencyTypeCRD,
	}

	// Determine if it's Git or HTTP based on --git-ref flag
	if c.GitRef != "" {
		// Git-based CRD
		if c.Package == "" {
			return errors.New("repository URL is required for git-based CRD dependencies")
		}
		c.apiDep.Git = &v2alpha1.APIGitReference{
			Repository: c.Package,
			Ref:        c.GitRef,
			Path:       c.GitPath,
		}
	} else {
		// HTTP-based CRD
		if c.Package == "" {
			return errors.New("URL is required for HTTP-based CRD dependencies")
		}
		if !strings.HasPrefix(c.Package, "http://") && !strings.HasPrefix(c.Package, "https://") {
			return errors.New("URL must start with http:// or https://")
		}
		c.apiDep.HTTP = &v2alpha1.APIHTTPReference{
			URL: c.Package,
		}
	}
	return nil
}

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *addCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	kongCtx.Bind(pterm.DefaultBulletList.WithWriter(kongCtx.Stdout))
	ctx := context.Background()

	// Build API dependency structure if --api flag is specified.
	if c.API {
		if err := c.parseAPIDependency(); err != nil {
			return err
		}
	}

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
		c.proj.Spec = &v2alpha1.ProjectSpec{}
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
	// Handle API dependencies - inferred from --api flag
	if c.API {
		return c.addAPIDependency(ctx, printer)
	}

	// Handle standard package dependencies
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

// addAPIDependency handles adding API dependencies to the project.
func (c *addCmd) addAPIDependency(ctx context.Context, printer upterm.ObjectPrinter) error {
	// Add the API dependency
	if err := upterm.WrapWithSuccessSpinner(
		"Adding API dependency...",
		upterm.CheckmarkSuccessSpinner,
		func() error {
			// Add to dependency manager (this also updates the project file)
			return c.m.AddAPIDependency(ctx, *c.apiDep)
		},
		printer,
	); err != nil {
		return err
	}

	processedAPIDeps, err := c.m.GetProcessedAPIDependencies(ctx, []v2alpha1.APIDependencies{*c.apiDep})
	if err != nil {
		return err
	}

	if len(processedAPIDeps) > 0 {
		dep := processedAPIDeps[0]
		pterm.Success.Printfln("API dependency added: %s (%s)", dep.Source, dep.Type)
	}

	return nil
}
