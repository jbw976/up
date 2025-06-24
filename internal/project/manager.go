// Copyright 2025 Upbound Inc.
// All rights reserved

package project

import (
	"context"

	"github.com/spf13/afero"
	"golang.org/x/sync/errgroup"
	"k8s.io/utils/ptr"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"

	"github.com/upbound/up/internal/schemas/generator"
	smanager "github.com/upbound/up/internal/schemas/manager"
	"github.com/upbound/up/internal/schemas/runner"
	"github.com/upbound/up/internal/upbound"
	ixpkg "github.com/upbound/up/internal/xpkg"
	"github.com/upbound/up/internal/xpkg/dep"
	"github.com/upbound/up/internal/xpkg/dep/cache"
	dmanager "github.com/upbound/up/internal/xpkg/dep/manager"
	"github.com/upbound/up/internal/xpkg/dep/marshaler/xpkg"
	"github.com/upbound/up/internal/xpkg/dep/resolver/image"
	"github.com/upbound/up/pkg/apis/project/v1alpha1"
)

// DependencyManager manages dependencies for a project, including both the xpkg
// cache and schemas.
type DependencyManager struct {
	proj     *v1alpha1.Project
	projFS   afero.Fs
	projFile string
	deps     *dmanager.Manager
	schemas  *smanager.Manager
}

// Add adds the given dependency to the project, caching and generating schemas
// for it and all its transitive dependencies.
func (m *DependencyManager) Add(ctx context.Context, d pkgmetav1.Dependency) error {
	c, ok := dmanager.ConvertToV1beta1(d)
	if !ok {
		return errors.New("invalid dependency")
	}
	_, pkgs, err := m.deps.AddAll(ctx, c)
	if err != nil {
		return errors.Wrap(err, "failed to add dependency to cache")
	}

	eg, egCtx := errgroup.WithContext(ctx)
	for _, pkg := range pkgs {
		eg.Go(func() error {
			s := smanager.NewXpkgSource(pkg)
			if err := m.schemas.Add(egCtx, s); err != nil {
				return errors.Wrapf(err, "failed to generate schemas for %q", pkg.Name())
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	if err := UpsertDependency(m.proj, d); err != nil {
		return errors.Wrap(err, "failed to add dependency to project")
	}
	if err := Update(m.projFS, m.projFile, func(p *v1alpha1.Project) {
		p.Spec.DependsOn = m.proj.Spec.DependsOn
	}); err != nil {
		return errors.Wrap(err, "failed to update project metadata")
	}

	return nil
}

// AddAll adds all the given dependencies.
func (m *DependencyManager) AddAll(ctx context.Context, ds ...pkgmetav1.Dependency) error {
	eg, egCtx := errgroup.WithContext(ctx)
	for _, d := range ds {
		eg.Go(func() error {
			if err := m.Add(egCtx, d); err != nil {
				d, _ = NormalizeDependency(d)
				return errors.Wrapf(err, "failed to add dependency %s", ptr.Deref(d.Package, ""))
			}
			return nil
		})
	}

	return eg.Wait()
}

// AddByRef adds a dependency by OCI ref.
func (m *DependencyManager) AddByRef(ctx context.Context, ref string) (pkgmetav1.Dependency, error) {
	if _, err := ixpkg.ValidDep(ref); err != nil {
		return pkgmetav1.Dependency{}, errors.Wrap(err, "invalid dependency")
	}

	// We don't know what kind of package the ref points to, so we can't just
	// call Add. Call the dependency manager's Add first to resolve the
	// dependency, then our add to make sure schemas get generated as well.
	parsed := dep.New(ref)
	resolved, _, err := m.deps.AddAll(ctx, parsed)
	if err != nil {
		return pkgmetav1.Dependency{}, errors.Wrap(err, "failed to resolve dependency")
	}

	// Retain the original constraints from the ref.
	//
	// TODO(adamwg): Consider changing this. Pinning dependency versions would
	// be a better practice.
	d := dep.ToMetaDependency(resolved)
	d.Version = parsed.Constraints

	return d, m.Add(ctx, d)
}

// GetParsedPackage returns a package from the dependency manager's cache. It
// returns an error if the package is not in the cache.
func (m *DependencyManager) GetParsedPackage(ctx context.Context, dep pkgmetav1.Dependency) (*xpkg.ParsedPackage, error) {
	d, ok := dmanager.ConvertToV1beta1(dep)
	if !ok {
		return nil, errors.New("invalid dependency")
	}
	view, err := m.deps.View(ctx, []v1beta1.Dependency{d})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get dependency view")
	}

	for name, pkg := range view.Packages() {
		if name == d.Package {
			return pkg, nil
		}
	}

	return nil, errors.New("package not found in cache")
}

// SchemaManager returns the schema manager.
func (m *DependencyManager) SchemaManager() *smanager.Manager {
	return m.schemas
}

// NewDependencyManager returns an initialized dependency manager.
func NewDependencyManager(upCtx *upbound.Context, proj *v1alpha1.Project, projFS afero.Fs, opts ...ManagerOption) (*DependencyManager, error) {
	options := &managerOptions{
		projFile: "upbound.yaml",
		fetcher:  image.NewLocalFetcher(image.WithKeychain(upCtx.RegistryKeychain())),
		schemaRunner: runner.NewRealSchemaRunner(
			runner.WithImageConfig(proj.Spec.ImageConfig),
		),
		schemaGenerators: generator.AllLanguages(),
		schemaFS:         afero.NewBasePathFs(projFS, ".up"),
		cacheFS:          afero.NewBasePathFs(afero.NewOsFs(), "~/.up/cache"),
	}

	for _, opt := range opts {
		opt(options)
	}

	cch, err := cache.NewLocal("/", cache.WithFS(options.cacheFS))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create xpkg cache")
	}

	res := image.NewResolver(
		image.WithImageConfig(proj.Spec.ImageConfig),
		image.WithFetcher(options.fetcher),
	)

	deps, err := dmanager.New(
		dmanager.WithCache(cch),
		dmanager.WithResolver(res),
		dmanager.WithSkipCacheUpdateIfExists(true),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create dependency manager")
	}

	schemas := smanager.New(
		options.schemaFS,
		options.schemaGenerators,
		options.schemaRunner,
	)

	return &DependencyManager{
		proj:     proj,
		projFS:   projFS,
		projFile: options.projFile,
		deps:     deps,
		schemas:  schemas,
	}, nil
}

type managerOptions struct {
	projFile         string
	schemaFS         afero.Fs
	cacheFS          afero.Fs
	fetcher          image.Fetcher
	schemaGenerators []generator.Interface
	schemaRunner     runner.SchemaRunner
}

// ManagerOption configures the dependency manager.
type ManagerOption func(*managerOptions)

// WithProjectFile sets the path to the project file within the project
// filesystem.
func WithProjectFile(path string) ManagerOption {
	return func(opts *managerOptions) {
		opts.projFile = path
	}
}

// WithSchemaFS sets the filesystem to use for schemas.
func WithSchemaFS(fs afero.Fs) ManagerOption {
	return func(opts *managerOptions) {
		opts.schemaFS = fs
	}
}

// WithCacheFS sets the filesystem to use for the xpkg cache.
func WithCacheFS(fs afero.Fs) ManagerOption {
	return func(opts *managerOptions) {
		opts.cacheFS = fs
	}
}

// WithFetcher sets the fetcher to use for fetching packages.
func WithFetcher(f image.Fetcher) ManagerOption {
	return func(opts *managerOptions) {
		opts.fetcher = f
	}
}

// WithSchemaRunner sets the runner to use when generating schemas.
func WithSchemaRunner(r runner.SchemaRunner) ManagerOption {
	return func(opts *managerOptions) {
		opts.schemaRunner = r
	}
}

// WithSchemaGenerators sets the schema generators to call.
func WithSchemaGenerators(gs []generator.Interface) ManagerOption {
	return func(opts *managerOptions) {
		opts.schemaGenerators = gs
	}
}

// NormalizeDependency converts dependencies to the modern format where
// APIVersion and Kind are specified.
func NormalizeDependency(dep pkgmetav1.Dependency) (pkgmetav1.Dependency, error) {
	if dep.APIVersion != nil && dep.Kind != nil && dep.Package != nil {
		return dep, nil
	}

	switch {
	case dep.Provider != nil:
		dep.APIVersion = ptr.To(pkgv1.ProviderGroupVersionKind.GroupVersion().String())
		dep.Kind = &pkgv1.ProviderKind
		dep.Package = dep.Provider
		dep.Provider = nil

	case dep.Function != nil:
		dep.APIVersion = ptr.To(pkgv1.FunctionGroupVersionKind.GroupVersion().String())
		dep.Kind = &pkgv1.FunctionKind
		dep.Package = dep.Function
		dep.Function = nil

	case dep.Configuration != nil:
		dep.APIVersion = ptr.To(pkgv1.ConfigurationGroupVersionKind.GroupVersion().String())
		dep.Kind = &pkgv1.ConfigurationKind
		dep.Package = dep.Configuration
		dep.Configuration = nil

	default:
		return dep, errors.New("unknown dependency type")
	}

	return dep, nil
}
