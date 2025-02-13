// Copyright 2025 Upbound Inc.
// All rights reserved

package v1alpha1

import (
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
)

func TestValidate(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		input          *Project
		expectedErrors []string
	}{
		"MinimalValid": {
			input: &Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-project",
				},
				Spec: &ProjectSpec{
					Repository: "xpkg.upbound.io/acmeco/my-project",
				},
			},
		},
		"MaximalValid": {
			input: &Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-project",
				},
				Spec: &ProjectSpec{
					Repository: "xpkg.upbound.io/acmeco/my-project",
					ProjectPackageMetadata: ProjectPackageMetadata{
						Maintainer:  "Acme Corporation",
						Source:      "https://github.com/acmeco/my-project.git",
						License:     "Apache-2.0",
						Description: "I'm a unit test",
						Readme:      "Don't use me, I'm a unit test",
					},
					Crossplane: &pkgmetav1.CrossplaneConstraints{
						Version: ">=1.17.0",
					},
					DependsOn: []pkgmetav1.Dependency{{
						Provider: ptr.To("xpkg.upbound.io/upbound/provider-aws-s3"),
						Version:  ">=0.2.1",
					}},
					Paths: &ProjectPaths{
						APIs:      "apis/",
						Functions: "functions/",
						Examples:  "examples/",
						Tests:     "tests/",
					},
					Architectures: []string{"arch1"},
				},
			},
		},
		"MissingName": {
			input: &Project{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: &ProjectSpec{
					Repository: "xpkg.upbound.io/acmeco/my-project",
				},
			},
			expectedErrors: []string{
				"name must not be empty",
			},
		},
		"MissingSpec": {
			input: &Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-project",
				},
			},
			expectedErrors: []string{
				"spec must be present",
			},
		},
		"MissingRepository": {
			input: &Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-project",
				},
				Spec: &ProjectSpec{},
			},
			expectedErrors: []string{
				"repository must not be empty",
			},
		},
		"AbsolutePaths": {
			input: &Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-project",
				},
				Spec: &ProjectSpec{
					Repository: "xpkg.upbound.io/acmeco/my-project",
					Paths: &ProjectPaths{
						APIs:      "/tmp/apis",
						Functions: "/tmp/functions",
						Examples:  "/tmp/examples",
						Tests:     "/tmp/tests",
					},
				},
			},
			expectedErrors: []string{
				"apis path must be relative",
				"functions path must be relative",
				"examples path must be relative",
			},
		},
		"EmptyArchitectures": {
			input: &Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-project",
				},
				Spec: &ProjectSpec{
					Repository:    "xpkg.upbound.io/acmeco/my-project",
					Architectures: []string{},
				},
			},
			expectedErrors: []string{
				"architectures must not be empty",
			},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := tc.input.Validate()
			if len(tc.expectedErrors) == 0 {
				assert.NilError(t, err)
			} else {
				for _, expected := range tc.expectedErrors {
					assert.Assert(t, cmp.ErrorContains(err, expected))
				}
			}
		})
	}
}

func TestDefault(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		input *Project
		want  *Project
	}{
		"FullySpecified": {
			input: &Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-project",
				},
				Spec: &ProjectSpec{
					Repository: "xpkg.upbound.io/acmeco/my-project",
					ProjectPackageMetadata: ProjectPackageMetadata{
						Maintainer:  "Acme Corporation",
						Source:      "https://github.com/acmeco/my-project.git",
						License:     "Apache-2.0",
						Description: "I'm a unit test",
						Readme:      "Don't use me, I'm a unit test",
					},
					Crossplane: &pkgmetav1.CrossplaneConstraints{
						Version: ">=1.17.0",
					},
					DependsOn: []pkgmetav1.Dependency{{
						Provider: ptr.To("xpkg.upbound.io/upbound/provider-aws-s3"),
						Version:  ">=0.2.1",
					}},
					Paths: &ProjectPaths{
						APIs:      "not-default-apis/",
						Functions: "not-default-functions/",
						Examples:  "not-default-examples/",
						Tests:     "not-default-tests/",
					},
					Architectures: []string{"arch1"},
				},
			},
			want: &Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-project",
				},
				Spec: &ProjectSpec{
					Repository: "xpkg.upbound.io/acmeco/my-project",
					ProjectPackageMetadata: ProjectPackageMetadata{
						Maintainer:  "Acme Corporation",
						Source:      "https://github.com/acmeco/my-project.git",
						License:     "Apache-2.0",
						Description: "I'm a unit test",
						Readme:      "Don't use me, I'm a unit test",
					},
					Crossplane: &pkgmetav1.CrossplaneConstraints{
						Version: ">=1.17.0",
					},
					DependsOn: []pkgmetav1.Dependency{{
						Provider: ptr.To("xpkg.upbound.io/upbound/provider-aws-s3"),
						Version:  ">=0.2.1",
					}},
					Paths: &ProjectPaths{
						APIs:      "not-default-apis/",
						Functions: "not-default-functions/",
						Examples:  "not-default-examples/",
						Tests:     "not-default-tests/",
					},
					Architectures: []string{"arch1"},
				},
			},
		},
		"MinimalValid": {
			input: &Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-project",
				},
				Spec: &ProjectSpec{
					Repository: "xpkg.upbound.io/acmeco/my-project",
				},
			},
			want: &Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-project",
				},
				Spec: &ProjectSpec{
					Repository: "xpkg.upbound.io/acmeco/my-project",
					Paths: &ProjectPaths{
						APIs:      "apis",
						Examples:  "examples",
						Functions: "functions",
						Tests:     "tests",
					},
					Architectures: []string{"amd64", "arm64"},
				},
			},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tc.input.Default()
			assert.DeepEqual(t, tc.want, tc.input)
		})
	}
}
