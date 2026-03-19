// Copyright 2025 Upbound Inc.
// All rights reserved

package dependency

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/alecthomas/kong"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/internal/xpkg/dep"
	"github.com/upbound/up/internal/xpkg/dep/cache"
	dmanager "github.com/upbound/up/internal/xpkg/dep/manager"
	xpkg "github.com/upbound/up/internal/xpkg/dep/marshaler/xpkg"
	"github.com/upbound/up/internal/xpkg/dep/resolver/image"
	"github.com/upbound/up/pkg/apis/project/v2alpha1"

	_ "embed"
)

//go:embed help/list.md
var listHelp string

func (c *listCmd) Help() string {
	return listHelp
}

var depListFieldNames = []string{"PACKAGE", "VERSION", "KIND"}

// listCmd lists all transitive dependencies for a project or a specific package.
type listCmd struct {
	Package     string `arg:"" optional:"" help:"Package reference to list dependencies for (e.g. xpkg.upbound.io/org/name:version). Defaults to current project."`
	ProjectFile string `default:"upbound.yaml" help:"Path to project definition file." short:"f"`
	CacheDir    string `default:"~/.up/cache/" env:"CACHE_DIR" help:"Directory used for caching package images." type:"path"`

	cch  dmanager.Cache
	res  *image.Resolver
	proj *v2alpha1.Project
}

// AfterApply constructs and binds context for the list command.
func (c *listCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	ctx := context.Background()

	cacheFS := afero.NewBasePathFs(afero.NewOsFs(), c.CacheDir)
	cch, err := cache.NewLocal("/", cache.WithFS(cacheFS))
	if err != nil {
		return errors.Wrap(err, "failed to create xpkg cache")
	}
	c.cch = cch

	var imageConfigs []v2alpha1.ImageConfig

	if c.Package == "" {
		projFilePath, err := filepath.Abs(c.ProjectFile)
		if err != nil {
			return err
		}
		projDirPath := filepath.Dir(projFilePath)
		projFS := afero.NewBasePathFs(afero.NewOsFs(), projDirPath)

		prj, err := project.Parse(projFS, c.ProjectFile)
		if err != nil {
			return errors.New("this is not a project directory")
		}
		c.proj = prj

		if prj.Spec != nil {
			imageConfigs = prj.Spec.ImageConfig
		}
	}

	c.res = image.NewResolver(
		image.WithImageConfig(imageConfigs),
		image.WithFetcher(image.NewLocalFetcher(image.WithKeychain(upCtx.RegistryKeychain()))),
	)

	kongCtx.BindTo(ctx, (*context.Context)(nil))
	return nil
}

// Run executes the list command.
func (c *listCmd) Run(ctx context.Context, printer upterm.Printer) error {
	if c.Package != "" {
		return c.runPackageMode(ctx, printer)
	}
	return c.runProjectMode(ctx, printer)
}

func (c *listCmd) runPackageMode(ctx context.Context, printer upterm.Printer) error {
	d := dep.New(c.Package)

	sp := printer.NewSuccessSpinner(fmt.Sprintf("Resolving %s...", d.Package))
	sp.Start()

	m, err := dmanager.New(
		dmanager.WithCache(c.cch),
		dmanager.WithResolver(c.res),
		dmanager.WithProgressFunc(func(pkg string) {
			sp.UpdateText(fmt.Sprintf("Resolving %s...", pkg))
		}),
	)
	if err != nil {
		sp.Fail()
		return errors.Wrap(err, "failed to create dependency manager")
	}

	_, pkgs, err := m.AddAll(ctx, d)
	if err != nil {
		sp.Fail()
		return fmt.Errorf("failed to resolve %s: %w", d.Package, err)
	}
	sp.Success()

	items := deduplicatePackages(pkgs)
	return printer.PrintObject(items, depListFieldNames, extractDepListFields)
}

func (c *listCmd) runProjectMode(ctx context.Context, printer upterm.Printer) error {
	if c.proj == nil || c.proj.Spec == nil || len(c.proj.Spec.DependsOn) == 0 {
		printer.Println("No dependencies found.")
		return nil
	}

	sp := printer.NewSuccessSpinner("Resolving dependencies...")
	sp.Start()

	m, err := dmanager.New(
		dmanager.WithCache(c.cch),
		dmanager.WithResolver(c.res),
		dmanager.WithProgressFunc(func(pkg string) {
			sp.UpdateText(fmt.Sprintf("Resolving %s...", pkg))
		}),
	)
	if err != nil {
		sp.Fail()
		return errors.Wrap(err, "failed to create dependency manager")
	}

	var allPkgs []*xpkg.ParsedPackage
	for _, d := range c.proj.Spec.DependsOn {
		converted, ok := dmanager.ConvertToV1beta1(d)
		if !ok {
			continue
		}
		_, pkgs, err := m.AddAll(ctx, converted)
		if err != nil {
			sp.Fail()
			return fmt.Errorf("failed to resolve %s: %w", converted.Package, err)
		}
		allPkgs = append(allPkgs, pkgs...)
	}
	sp.Success()

	items := deduplicatePackages(allPkgs)
	return printer.PrintObject(items, depListFieldNames, extractDepListFields)
}

// depListItem is a single entry in the flat dependency list.
type depListItem struct {
	Package string `json:"package" yaml:"package"`
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
	Kind    string `json:"kind,omitempty" yaml:"kind,omitempty"`
}

// deduplicatePackages converts a (possibly duplicated) flat package slice into
// a sorted, deduplicated []depListItem.
func deduplicatePackages(pkgs []*xpkg.ParsedPackage) []depListItem {
	seen := make(map[string]depListItem, len(pkgs))
	for _, pkg := range pkgs {
		if pkg == nil {
			continue
		}
		if _, exists := seen[pkg.Name()]; !exists {
			seen[pkg.Name()] = depListItem{
				Package: pkg.Name(),
				Version: pkg.Version(),
				Kind:    pkg.PKind(),
			}
		}
	}

	items := make([]depListItem, 0, len(seen))
	for _, item := range seen {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Package < items[j].Package
	})
	return items
}

// extractDepListFields extracts table columns from a depListItem.
func extractDepListFields(obj any) []string {
	d, ok := obj.(depListItem)
	if !ok {
		return []string{"", "", ""}
	}
	return []string{d.Package, d.Version, d.Kind}
}
