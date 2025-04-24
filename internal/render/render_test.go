// Copyright 2025 Upbound Inc.
// All rights reserved

package render

import (
	"context"
	"testing"

	"gotest.tools/v3/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	metav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"

	image "github.com/upbound/up/internal/xpkg/dep/resolver/image"
	projectv1alpha1 "github.com/upbound/up/pkg/apis/project/v1alpha1"
)

func TestLoadFunctions(t *testing.T) {
	// Define mock data and behavior
	type testCase struct {
		proj                 *projectv1alpha1.Project
		expectedFunctions    []pkgv1.Function
		expectedErrorMessage string
		fetcher              image.Fetcher
	}

	tests := map[string]testCase{
		"SuccessfullOneFunction": {
			proj: &projectv1alpha1.Project{
				Spec: &projectv1alpha1.ProjectSpec{
					DependsOn: []metav1.Dependency{
						{
							Function: ptr.To("registry.example.com/library/function-1"),
							Version:  ">=v0.0.0",
						},
					},
				},
			},
			expectedFunctions: []pkgv1.Function{
				{
					ObjectMeta: v1.ObjectMeta{Name: "library-function-1"},
					Spec: pkgv1.FunctionSpec{
						PackageSpec: pkgv1.PackageSpec{
							Package: "registry.example.com/library/function-1:v1.0.0",
						},
					},
				},
			},
			fetcher: image.NewMockFetcher(
				image.WithTags(
					[]string{
						"v1.0.0",
					},
				),
			),
		},
		"SuccessfullMultipleFunctions": {
			proj: &projectv1alpha1.Project{
				Spec: &projectv1alpha1.ProjectSpec{
					DependsOn: []metav1.Dependency{
						{
							Function: ptr.To("registry.example.com/library/function-1"),
							Version:  ">=v0.0.0",
						},
						{
							Function: ptr.To("registry.example.com/library/function-2"),
							Version:  ">=v0.0.0",
						},
					},
				},
			},
			expectedFunctions: []pkgv1.Function{
				{
					ObjectMeta: v1.ObjectMeta{Name: "library-function-1"},
					Spec: pkgv1.FunctionSpec{
						PackageSpec: pkgv1.PackageSpec{
							Package: "registry.example.com/library/function-1:v2.0.0",
						},
					},
				},
				{
					ObjectMeta: v1.ObjectMeta{Name: "library-function-2"},
					Spec: pkgv1.FunctionSpec{
						PackageSpec: pkgv1.PackageSpec{
							Package: "registry.example.com/library/function-2:v2.0.0",
						},
					},
				},
			},
			fetcher: image.NewMockFetcher(
				image.WithTags(
					[]string{
						"v1.0.0",
						"v2.0.0",
					},
				),
			),
		},
		"SuccessfullMultipleFunctionsAndProvider": {
			proj: &projectv1alpha1.Project{
				Spec: &projectv1alpha1.ProjectSpec{
					DependsOn: []metav1.Dependency{
						{
							Function: ptr.To("registry.example.com/library/function-1"),
							Version:  ">=v0.0.0",
						},
						{
							Function: ptr.To("registry.example.com/library/function-2"),
							Version:  ">=v0.0.0",
						},
						{
							Provider: ptr.To("registry.example.com/library/provider-1"),
							Version:  ">=v0.0.0",
						},
					},
				},
			},
			expectedFunctions: []pkgv1.Function{
				{
					ObjectMeta: v1.ObjectMeta{Name: "library-function-1"},
					Spec: pkgv1.FunctionSpec{
						PackageSpec: pkgv1.PackageSpec{
							Package: "registry.example.com/library/function-1:v2.0.0",
						},
					},
				},
				{
					ObjectMeta: v1.ObjectMeta{Name: "library-function-2"},
					Spec: pkgv1.FunctionSpec{
						PackageSpec: pkgv1.PackageSpec{
							Package: "registry.example.com/library/function-2:v2.0.0",
						},
					},
				},
			},
			fetcher: image.NewMockFetcher(
				image.WithTags(
					[]string{
						"v1.0.0",
						"v2.0.0",
					},
				),
			),
		},
		"SuccessfullMultipleFunctionsAndConfiguration": {
			proj: &projectv1alpha1.Project{
				Spec: &projectv1alpha1.ProjectSpec{
					DependsOn: []metav1.Dependency{
						{
							Function: ptr.To("registry.example.com/library/function-1"),
							Version:  ">=v0.0.0",
						},
						{
							Function: ptr.To("registry.example.com/library/function-2"),
							Version:  ">=v0.0.0",
						},
						{
							Configuration: ptr.To("registry.example.com/library/cfg-1"),
							Version:       ">=v0.0.0",
						},
					},
				},
			},
			expectedFunctions: []pkgv1.Function{
				{
					ObjectMeta: v1.ObjectMeta{Name: "library-function-1"},
					Spec: pkgv1.FunctionSpec{
						PackageSpec: pkgv1.PackageSpec{
							Package: "registry.example.com/library/function-1:v2.0.0",
						},
					},
				},
				{
					ObjectMeta: v1.ObjectMeta{Name: "library-function-2"},
					Spec: pkgv1.FunctionSpec{
						PackageSpec: pkgv1.PackageSpec{
							Package: "registry.example.com/library/function-2:v2.0.0",
						},
					},
				},
			},
			fetcher: image.NewMockFetcher(
				image.WithTags(
					[]string{
						"v1.0.0",
						"v2.0.0",
					},
				),
			),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			functions, err := loadFunctions(context.Background(), tc.proj, image.NewResolver(image.WithFetcher(tc.fetcher)))

			if tc.expectedErrorMessage != "" {
				assert.ErrorContains(t, err, tc.expectedErrorMessage)
			} else {
				assert.NilError(t, err)
				assert.DeepEqual(t, tc.expectedFunctions, functions)
			}
		})
	}
}
