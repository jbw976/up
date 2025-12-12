// Copyright 2025 Upbound Inc.
// All rights reserved

package dependency

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/spf13/afero"
	"k8s.io/utils/ptr"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	pkgmetav1 "github.com/crossplane/crossplane/v2/apis/pkg/meta/v1"

	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/pkg/apis/project/v2alpha1"

	_ "embed"
)

//go:embed help/add.md
var addHelp string

func (c *addCmd) Help() string {
	return addHelp
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

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *addCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
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
func (c *addCmd) Run(ctx context.Context, printer upterm.Printer) error {
	// Handle API dependencies - inferred from --api flag
	if c.API {
		return c.addAPIDependency(ctx, printer)
	}

	// Handle standard package dependencies
	var d pkgmetav1.Dependency
	if err := printer.WrapWithSuccessSpinner(
		"Adding dependency...",
		func() error {
			var err error
			d, err = c.m.AddByRef(ctx, c.Package)
			return err
		},
	); err != nil {
		return err
	}
	printer.PrintSuccess(fmt.Sprintf("%s:%s added", ptr.Deref(d.Package, ""), d.Version))

	return nil
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

// addAPIDependency handles adding API dependencies to the project.
func (c *addCmd) addAPIDependency(ctx context.Context, printer upterm.Printer) error {
	// Add the API dependency
	if err := printer.WrapWithSuccessSpinner(
		"Adding API dependency...",
		func() error {
			// Add to dependency manager (this also updates the project file)
			return c.m.AddAPIDependency(ctx, *c.apiDep)
		},
	); err != nil {
		return err
	}

	processedAPIDeps, err := c.m.GetProcessedAPIDependencies(ctx, []v2alpha1.APIDependencies{*c.apiDep})
	if err != nil {
		return err
	}

	if len(processedAPIDeps) > 0 {
		dep := processedAPIDeps[0]
		printer.PrintSuccess(fmt.Sprintf("API dependency added: %s (%s)", dep.Source, dep.Type))
	}

	return nil
}
