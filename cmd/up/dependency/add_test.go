// Copyright 2025 Upbound Inc.
// All rights reserved

package dependency

import (
	"context"
	"embed"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/spf13/afero"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	pkgv1beta1 "github.com/crossplane/crossplane/apis/pkg/v1beta1"

	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/internal/xpkg/dep/cache"
	"github.com/upbound/up/internal/xpkg/dep/manager"
	"github.com/upbound/up/internal/xpkg/dep/resolver/image"
	"github.com/upbound/up/pkg/apis/project/v1alpha1"
)

//go:embed testdata/packages/*
var packagesFS embed.FS

type addTestCase struct {
	inputDeps    []pkgmetav1.Dependency
	newPackage   string
	imageTag     name.Tag
	packageType  pkgv1beta1.PackageType
	expectedDeps []pkgmetav1.Dependency
	expectError  bool // Add this field to indicate whether an error is expected
}

func TestAdd(t *testing.T) {
	t.Parallel()

	tcs := map[string]addTestCase{
		"AddFunctionWithoutVersion": {
			inputDeps:   nil,
			newPackage:  "xpkg.upbound.io/crossplane-contrib/function-auto-ready",
			imageTag:    name.MustParseReference("xpkg.upbound.io/crossplane-contrib/function-auto-ready:v0.2.1").(name.Tag),
			packageType: pkgv1beta1.FunctionPackageType,
			expectedDeps: []pkgmetav1.Dependency{{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Function"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/function-auto-ready"),
				Version:    ">=v0.0.0",
			}},
		},
		"AddProviderWithoutVersion": {
			inputDeps:   nil,
			newPackage:  "xpkg.upbound.io/crossplane-contrib/provider-nop",
			imageTag:    name.MustParseReference("xpkg.upbound.io/crossplane-contrib/provider-nop:v0.2.1").(name.Tag),
			packageType: pkgv1beta1.ProviderPackageType,
			expectedDeps: []pkgmetav1.Dependency{{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Provider"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/provider-nop"),
				Version:    ">=v0.0.0",
			}},
		},
		"AddConfigurationWithoutVersion": {
			inputDeps:   nil,
			newPackage:  "xpkg.upbound.io/crossplane-contrib/configuration-empty",
			imageTag:    name.MustParseReference("xpkg.upbound.io/crossplane-contrib/configuration-empty:v0.1.0").(name.Tag),
			packageType: pkgv1beta1.ConfigurationPackageType,
			expectedDeps: []pkgmetav1.Dependency{{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Configuration"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/configuration-empty"),
				Version:    ">=v0.0.0",
			}},
		},
		"AddFunctionWithVersion": {
			inputDeps:   nil,
			newPackage:  "xpkg.upbound.io/crossplane-contrib/function-auto-ready:v0.2.1",
			imageTag:    name.MustParseReference("xpkg.upbound.io/crossplane-contrib/function-auto-ready:v0.2.1").(name.Tag),
			packageType: pkgv1beta1.FunctionPackageType,
			expectedDeps: []pkgmetav1.Dependency{{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Function"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/function-auto-ready"),
				Version:    "v0.2.1",
			}},
		},
		"AddFunctionWithSemVersion": {
			inputDeps:   nil,
			newPackage:  "xpkg.upbound.io/crossplane-contrib/function-auto-ready@>=v0.1.0",
			imageTag:    name.MustParseReference("xpkg.upbound.io/crossplane-contrib/function-auto-ready:v0.2.1").(name.Tag),
			packageType: pkgv1beta1.FunctionPackageType,
			expectedDeps: []pkgmetav1.Dependency{{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Function"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/function-auto-ready"),
				Version:    ">=v0.1.0",
			}},
		},
		"AddFunctionWithSemVersionGreaterThan": {
			inputDeps:   nil,
			newPackage:  "xpkg.upbound.io/crossplane-contrib/function-auto-ready@>v0.1.0",
			imageTag:    name.MustParseReference("xpkg.upbound.io/crossplane-contrib/function-auto-ready:v0.2.1").(name.Tag),
			packageType: pkgv1beta1.FunctionPackageType,
			expectedDeps: []pkgmetav1.Dependency{{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Function"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/function-auto-ready"),
				Version:    ">v0.1.0",
			}},
		},
		"AddFunctionWithSemVersionLessThan": {
			inputDeps:   nil,
			newPackage:  "xpkg.upbound.io/crossplane-contrib/function-auto-ready@<v0.3.0",
			imageTag:    name.MustParseReference("xpkg.upbound.io/crossplane-contrib/function-auto-ready:v0.2.1").(name.Tag),
			packageType: pkgv1beta1.FunctionPackageType,
			expectedDeps: []pkgmetav1.Dependency{{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Function"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/function-auto-ready"),
				Version:    "<v0.3.0",
			}},
		},
		"AddFunctionWithSemVersionLessThanError": {
			inputDeps:    nil,
			newPackage:   "xpkg.upbound.io/crossplane-contrib/function-auto-ready@<v0.2.0",
			imageTag:     name.MustParseReference("xpkg.upbound.io/crossplane-contrib/function-auto-ready:v0.2.1").(name.Tag),
			packageType:  pkgv1beta1.FunctionPackageType,
			expectedDeps: nil,  // No dependencies should be added because of the version mismatch.
			expectError:  true, // Add this field to indicate this test expects an error.
		},
		"AddProviderWithVersion": {
			inputDeps:   nil,
			newPackage:  "xpkg.upbound.io/crossplane-contrib/provider-nop@v0.2.1",
			imageTag:    name.MustParseReference("xpkg.upbound.io/crossplane-contrib/provider-nop:v0.2.1").(name.Tag),
			packageType: pkgv1beta1.ProviderPackageType,
			expectedDeps: []pkgmetav1.Dependency{{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Provider"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/provider-nop"),
				Version:    "v0.2.1",
			}},
		},
		"AddProviderWithSemVersion": {
			inputDeps:   nil,
			newPackage:  "xpkg.upbound.io/crossplane-contrib/provider-nop@<=v0.3.0",
			imageTag:    name.MustParseReference("xpkg.upbound.io/crossplane-contrib/provider-nop:v0.2.1").(name.Tag),
			packageType: pkgv1beta1.ProviderPackageType,
			expectedDeps: []pkgmetav1.Dependency{{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Provider"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/provider-nop"),
				Version:    "<=v0.3.0",
			}},
		},
		"AddConfigurationWithVersion": {
			inputDeps:   nil,
			newPackage:  "xpkg.upbound.io/crossplane-contrib/configuration-empty@v0.1.0",
			imageTag:    name.MustParseReference("xpkg.upbound.io/crossplane-contrib/configuration-empty:v0.1.0").(name.Tag),
			packageType: pkgv1beta1.ConfigurationPackageType,
			expectedDeps: []pkgmetav1.Dependency{{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Configuration"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/configuration-empty"),
				Version:    "v0.1.0",
			}},
		},
		"AddConfigurationWithSemVersion": {
			inputDeps:   nil,
			newPackage:  "xpkg.upbound.io/crossplane-contrib/configuration-empty@<=v0.1.0",
			imageTag:    name.MustParseReference("xpkg.upbound.io/crossplane-contrib/configuration-empty:v0.1.0").(name.Tag),
			packageType: pkgv1beta1.ConfigurationPackageType,
			expectedDeps: []pkgmetav1.Dependency{{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Configuration"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/configuration-empty"),
				Version:    "<=v0.1.0",
			}},
		},
		"AddConfigurationWithSemVersionNotAvailable": {
			inputDeps:    nil,
			newPackage:   "xpkg.upbound.io/crossplane-contrib/configuration-empty@>=v1.0.0",
			imageTag:     name.MustParseReference("xpkg.upbound.io/crossplane-contrib/configuration-empty:v0.1.0").(name.Tag),
			packageType:  pkgv1beta1.ConfigurationPackageType,
			expectedDeps: nil,  // No dependencies should be added because of the version mismatch.
			expectError:  true, // Add this field to indicate this test expects an error.
		},
		"AddProviderWithExistingDeps": {
			inputDeps: []pkgmetav1.Dependency{{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Function"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/function-auto-ready"),
				Version:    "v0.2.1",
			}},
			newPackage:  "xpkg.upbound.io/crossplane-contrib/provider-nop@v0.2.1",
			imageTag:    name.MustParseReference("xpkg.upbound.io/crossplane-contrib/provider-nop:v0.2.1").(name.Tag),
			packageType: pkgv1beta1.ProviderPackageType,
			expectedDeps: []pkgmetav1.Dependency{
				{
					APIVersion: ptr.To("pkg.crossplane.io/v1"),
					Kind:       ptr.To("Function"),
					Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/function-auto-ready"),
					Version:    "v0.2.1",
				},
				{
					APIVersion: ptr.To("pkg.crossplane.io/v1"),
					Kind:       ptr.To("Provider"),
					Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/provider-nop"),
					Version:    "v0.2.1",
				},
			},
		},
		"UpdateFunction": {
			inputDeps: []pkgmetav1.Dependency{{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Function"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/function-auto-ready"),
				Version:    "v0.1.0",
			}},
			newPackage:  "xpkg.upbound.io/crossplane-contrib/function-auto-ready:v0.2.1",
			imageTag:    name.MustParseReference("xpkg.upbound.io/crossplane-contrib/function-auto-ready:v0.2.1").(name.Tag),
			packageType: pkgv1beta1.FunctionPackageType,
			expectedDeps: []pkgmetav1.Dependency{{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Function"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/function-auto-ready"),
				Version:    "v0.2.1",
			}},
		},
		"AddFunctionWithVersionColon": {
			inputDeps:   nil,
			newPackage:  "xpkg.upbound.io/crossplane-contrib/function-auto-ready:v0.2.1",
			imageTag:    name.MustParseReference("xpkg.upbound.io/crossplane-contrib/function-auto-ready:v0.2.1").(name.Tag),
			packageType: pkgv1beta1.FunctionPackageType,
			expectedDeps: []pkgmetav1.Dependency{{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Function"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/function-auto-ready"),
				Version:    "v0.2.1",
			}},
		},
		"AddProviderWithVersionColon": {
			inputDeps:   nil,
			newPackage:  "xpkg.upbound.io/crossplane-contrib/provider-nop:v0.2.1",
			imageTag:    name.MustParseReference("xpkg.upbound.io/crossplane-contrib/provider-nop:v0.2.1").(name.Tag),
			packageType: pkgv1beta1.ProviderPackageType,
			expectedDeps: []pkgmetav1.Dependency{{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Provider"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/provider-nop"),
				Version:    "v0.2.1",
			}},
		},
		"AddConfigurationWithVersionColon": {
			inputDeps:   nil,
			newPackage:  "xpkg.upbound.io/crossplane-contrib/configuration-empty:v0.1.0",
			imageTag:    name.MustParseReference("xpkg.upbound.io/crossplane-contrib/configuration-empty:v0.1.0").(name.Tag),
			packageType: pkgv1beta1.ConfigurationPackageType,
			expectedDeps: []pkgmetav1.Dependency{{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Configuration"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/configuration-empty"),
				Version:    "v0.1.0",
			}},
		},
		"AddProviderWithExistingDepsColon": {
			inputDeps: []pkgmetav1.Dependency{{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Function"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/function-auto-ready"),
				Version:    "v0.2.1",
			}},
			newPackage:  "xpkg.upbound.io/crossplane-contrib/provider-nop:v0.2.1",
			imageTag:    name.MustParseReference("xpkg.upbound.io/crossplane-contrib/provider-nop:v0.2.1").(name.Tag),
			packageType: pkgv1beta1.ProviderPackageType,
			expectedDeps: []pkgmetav1.Dependency{
				{
					APIVersion: ptr.To("pkg.crossplane.io/v1"),
					Kind:       ptr.To("Function"),
					Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/function-auto-ready"),
					Version:    "v0.2.1",
				},
				{
					APIVersion: ptr.To("pkg.crossplane.io/v1"),
					Kind:       ptr.To("Provider"),
					Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/provider-nop"),
					Version:    "v0.2.1",
				},
			},
		},
		"UpdateFunctionColon": {
			inputDeps: []pkgmetav1.Dependency{{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Function"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/function-auto-ready"),
				Version:    "v0.1.0",
			}},
			newPackage:  "xpkg.upbound.io/crossplane-contrib/function-auto-ready:v0.2.1",
			imageTag:    name.MustParseReference("xpkg.upbound.io/crossplane-contrib/function-auto-ready:v0.2.1").(name.Tag),
			packageType: pkgv1beta1.FunctionPackageType,
			expectedDeps: []pkgmetav1.Dependency{{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Function"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/function-auto-ready"),
				Version:    "v0.2.1",
			}},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tc.Run(t)
		})
	}
}

func (tc *addTestCase) Run(t *testing.T) {
	fs := afero.NewMemMapFs()
	inputProj := &v1alpha1.Project{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.ProjectGroupVersionKind.GroupVersion().String(),
			Kind:       v1alpha1.ProjectKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-project",
		},
		Spec: &v1alpha1.ProjectSpec{
			Repository: "xpkg.upbound.io/test/test",
			DependsOn:  tc.inputDeps,
		},
	}
	bs, err := yaml.Marshal(inputProj)
	assert.NilError(t, err)
	err = afero.WriteFile(fs, "upbound.yaml", bs, 0o644)
	assert.NilError(t, err)

	cch, err := cache.NewLocal("/cache", cache.WithFS(fs))
	assert.NilError(t, err)

	testPkgFS := afero.NewBasePathFs(afero.FromIOFS{FS: packagesFS}, "testdata/packages")

	r := image.NewResolver(
		image.WithFetcher(
			&image.FSFetcher{FS: testPkgFS},
		),
	)

	mgr, err := manager.New(
		manager.WithCache(cch),
		manager.WithResolver(r),
	)
	assert.NilError(t, err)

	// Add the dependency.
	printer := upterm.DefaultObjPrinter
	printer.Quiet = true
	cmd := &addCmd{
		m:           mgr,
		proj:        inputProj,
		projFS:      fs,
		ProjectFile: "upbound.yaml",
		Package:     tc.newPackage,
	}
	err = cmd.Run(context.Background(), printer)

	// Check if we expect an error.
	if tc.expectError {
		assert.ErrorContains(t, err, "supplied version does not match an existing version")
		return // No need to proceed with further checks if this is an error case.
	}
	assert.NilError(t, err)

	// Verify that the dep was correctly added to the metadata.
	updatedBytes, err := afero.ReadFile(fs, "upbound.yaml")
	assert.NilError(t, err)

	var updatedProj v1alpha1.Project
	err = yaml.Unmarshal(updatedBytes, &updatedProj)
	assert.NilError(t, err)
	assert.DeepEqual(t, tc.expectedDeps, updatedProj.Spec.DependsOn)

	// Verify that the dep was added to the cache.
	cchPkg, err := cch.Get(pkgv1beta1.Dependency{
		Package:     tc.imageTag.RegistryStr() + "/" + tc.imageTag.RepositoryStr(),
		Type:        &tc.packageType,
		Constraints: tc.imageTag.TagStr(),
	})
	assert.NilError(t, err)
	assert.Assert(t, cchPkg != nil)
}
