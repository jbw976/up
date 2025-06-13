// Copyright 2025 Upbound Inc.
// All rights reserved

package project

import (
	"io/fs"
	"testing"

	"github.com/spf13/afero"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"

	"github.com/upbound/up/internal/yaml"
	"github.com/upbound/up/pkg/apis/project/v1alpha1"
)

func TestUpsertDependency(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		inputDeps    []pkgmetav1.Dependency
		newDep       pkgmetav1.Dependency
		expectedDeps []pkgmetav1.Dependency
	}{
		"AddFunctionWithVersion": {
			inputDeps: nil,
			newDep: pkgmetav1.Dependency{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Function"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/function-auto-ready"),
				Version:    "v0.2.1",
			},
			expectedDeps: []pkgmetav1.Dependency{{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Function"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/function-auto-ready"),
				Version:    "v0.2.1",
			}},
		},
		"AddFunctionWithSemVersion": {
			inputDeps: nil,
			newDep: pkgmetav1.Dependency{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Function"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/function-auto-ready"),
				Version:    ">=v0.1.0",
			},
			expectedDeps: []pkgmetav1.Dependency{{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Function"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/function-auto-ready"),
				Version:    ">=v0.1.0",
			}},
		},
		"AddFunctionWithSemVersionGreaterThan": {
			inputDeps: nil,
			newDep: pkgmetav1.Dependency{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Function"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/function-auto-ready"),
				Version:    ">v0.1.0",
			},
			expectedDeps: []pkgmetav1.Dependency{{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Function"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/function-auto-ready"),
				Version:    ">v0.1.0",
			}},
		},
		"AddFunctionWithSemVersionLessThan": {
			inputDeps: nil,
			newDep: pkgmetav1.Dependency{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Function"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/function-auto-ready"),
				Version:    "<v0.3.0",
			},
			expectedDeps: []pkgmetav1.Dependency{{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Function"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/function-auto-ready"),
				Version:    "<v0.3.0",
			}},
		},
		"AddProviderWithVersion": {
			inputDeps: nil,
			newDep: pkgmetav1.Dependency{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Provider"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/provider-nop"),
				Version:    "v0.2.1",
			},
			expectedDeps: []pkgmetav1.Dependency{{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Provider"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/provider-nop"),
				Version:    "v0.2.1",
			}},
		},
		"AddProviderWithSemVersion": {
			inputDeps: nil,
			newDep: pkgmetav1.Dependency{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Provider"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/provider-nop"),
				Version:    "<=v0.3.0",
			},
			expectedDeps: []pkgmetav1.Dependency{{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Provider"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/provider-nop"),
				Version:    "<=v0.3.0",
			}},
		},
		"AddConfigurationWithVersion": {
			inputDeps: nil,
			newDep: pkgmetav1.Dependency{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Configuration"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/configuration-empty"),
				Version:    "v0.1.0",
			},
			expectedDeps: []pkgmetav1.Dependency{{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Configuration"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/configuration-empty"),
				Version:    "v0.1.0",
			}},
		},
		"AddConfigurationWithSemVersion": {
			inputDeps: nil,
			newDep: pkgmetav1.Dependency{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Configuration"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/configuration-empty"),
				Version:    "<=v0.1.0",
			},
			expectedDeps: []pkgmetav1.Dependency{{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Configuration"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/configuration-empty"),
				Version:    "<=v0.1.0",
			}},
		},
		"AddProviderWithExistingDeps": {
			inputDeps: []pkgmetav1.Dependency{{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Function"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/function-auto-ready"),
				Version:    "v0.2.1",
			}},
			newDep: pkgmetav1.Dependency{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Provider"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/provider-nop"),
				Version:    "v0.2.1",
			},
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
			newDep: pkgmetav1.Dependency{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Function"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/function-auto-ready"),
				Version:    "v0.2.1",
			},
			expectedDeps: []pkgmetav1.Dependency{{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Function"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/function-auto-ready"),
				Version:    "v0.2.1",
			}},
		},
		"AddFunctionWithOldStyle": {
			inputDeps: nil,
			newDep: pkgmetav1.Dependency{
				Function: ptr.To("xpkg.upbound.io/crossplane-contrib/function-auto-ready"),
				Version:  "v0.2.1",
			},
			expectedDeps: []pkgmetav1.Dependency{{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Function"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/function-auto-ready"),
				Version:    "v0.2.1",
			}},
		},
		"AddProviderWithOldStyle": {
			inputDeps: nil,
			newDep: pkgmetav1.Dependency{
				Provider: ptr.To("xpkg.upbound.io/crossplane-contrib/provider-nop"),
				Version:  "v0.2.1",
			},
			expectedDeps: []pkgmetav1.Dependency{{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Provider"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/provider-nop"),
				Version:    "v0.2.1",
			}},
		},
		"AddConfigurationWithOldStyle": {
			inputDeps: nil,
			newDep: pkgmetav1.Dependency{
				Configuration: ptr.To("xpkg.upbound.io/crossplane-contrib/configuration-empty"),
				Version:       "v0.1.0",
			},
			expectedDeps: []pkgmetav1.Dependency{{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Configuration"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/configuration-empty"),
				Version:    "v0.1.0",
			}},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			proj := &v1alpha1.Project{
				TypeMeta: metav1.TypeMeta{
					APIVersion: v1alpha1.ProjectGroupVersionKind.GroupVersion().String(),
					Kind:       v1alpha1.ProjectKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-project",
				},
				Spec: &v1alpha1.ProjectSpec{
					DependsOn: tc.inputDeps,
				},
			}

			err := UpsertDependency(proj, tc.newDep)
			assert.NilError(t, err)
			assert.DeepEqual(t, tc.expectedDeps, proj.Spec.DependsOn)
		})
	}
}

func TestUpdate(t *testing.T) {
	t.Parallel()

	// Write the original project.
	proj := &v1alpha1.Project{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.ProjectGroupVersionKind.GroupVersion().String(),
			Kind:       v1alpha1.ProjectKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-project",
		},
		Spec: &v1alpha1.ProjectSpec{
			Repository: "xpkg.upbound.io/foo/bar",
		},
	}
	bs, err := yaml.Marshal(proj)
	assert.NilError(t, err)
	projFS := afero.NewMemMapFs()
	afero.WriteFile(projFS, "upbound.yaml", bs, 0o666)

	// Update!
	var want []byte
	err = Update(projFS, "upbound.yaml", func(p *v1alpha1.Project) {
		// Update the project and marshal it so we know what to expect on disk.
		p.Spec.Architectures = []string{"arch1", "arch2"}
		p.Spec.Paths = &v1alpha1.ProjectPaths{
			APIs:      "my-cool-apis",
			Functions: "my-cool-functions",
			Examples:  "my-cool-examples",
			Tests:     "my-cool-tests",
		}
		p.Spec.DependsOn = []pkgmetav1.Dependency{{
			APIVersion: ptr.To("pkg.crossplane.io/v1"),
			Kind:       ptr.To("Provider"),
			Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/provider-nop"),
			Version:    "v0.2.1",
		}}
		want, err = yaml.Marshal(p)
		assert.NilError(t, err)
	})
	assert.NilError(t, err)

	// Verify the contents of the file was updated correctly.
	got, err := afero.ReadFile(projFS, "upbound.yaml")
	assert.NilError(t, err)
	assert.DeepEqual(t, want, got)

	// Verify that the permissions were retained.
	st, err := projFS.Stat("upbound.yaml")
	assert.NilError(t, err)
	assert.Equal(t, st.Mode(), fs.FileMode(0o666))
}
