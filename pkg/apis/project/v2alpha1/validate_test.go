// Copyright 2025 Upbound Inc.
// All rights reserved

package v2alpha1

import (
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	pkgmetav1 "github.com/crossplane/crossplane/v2/apis/pkg/meta/v1"
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
						APIs:       "apis/",
						Functions:  "functions/",
						Examples:   "examples/",
						Tests:      "tests/",
						Operations: "operations/",
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
						APIs:       "/tmp/apis",
						Functions:  "/tmp/functions",
						Examples:   "/tmp/examples",
						Tests:      "/tmp/tests",
						Operations: "/tmp/operations",
					},
				},
			},
			expectedErrors: []string{
				"apis path must be relative",
				"functions path must be relative",
				"examples path must be relative",
				"tests path must be relative",
				"operations path must be relative",
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
		"ValidAPIDependency": {
			input: &Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-project",
				},
				Spec: &ProjectSpec{
					Repository: "xpkg.upbound.io/acmeco/my-project",
					APIDependencies: []APIDependencies{
						{
							Type: "crd",
							Git: &APIGitReference{
								Repository: "https://github.com/crossplane/crossplane.git",
								Ref:        "v1.14.0",
								Path:       "cluster/crds",
							},
						},
					},
				},
			},
		},
		"InvalidAPIDependencyNoType": {
			input: &Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-project",
				},
				Spec: &ProjectSpec{
					Repository: "xpkg.upbound.io/acmeco/my-project",
					APIDependencies: []APIDependencies{
						{
							Git: &APIGitReference{
								Repository: "https://github.com/crossplane/crossplane.git",
							},
						},
					},
				},
			},
			expectedErrors: []string{
				"api dependency 0: type must not be empty",
			},
		},
		"InvalidAPIDependencyNoSource": {
			input: &Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-project",
				},
				Spec: &ProjectSpec{
					Repository: "xpkg.upbound.io/acmeco/my-project",
					APIDependencies: []APIDependencies{
						{
							Type: "crd",
						},
					},
				},
			},
			expectedErrors: []string{
				"api dependency 0: exactly one source (git, http, or k8s) must be specified",
			},
		},
		"InvalidAPIDependencyMultipleSources": {
			input: &Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-project",
				},
				Spec: &ProjectSpec{
					Repository: "xpkg.upbound.io/acmeco/my-project",
					APIDependencies: []APIDependencies{
						{
							Type: "crd",
							Git: &APIGitReference{
								Repository: "https://github.com/crossplane/crossplane.git",
							},
							HTTP: &APIHTTPReference{
								URL: "https://example.com/api.yaml",
							},
						},
					},
				},
			},
			expectedErrors: []string{
				"api dependency 0: only one source (git, http, or k8s) may be specified",
			},
		},
		"InvalidAPIDependencyGitEmptyRepository": {
			input: &Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-project",
				},
				Spec: &ProjectSpec{
					Repository: "xpkg.upbound.io/acmeco/my-project",
					APIDependencies: []APIDependencies{
						{
							Type: "crd",
							Git: &APIGitReference{
								Repository: "",
							},
						},
					},
				},
			},
			expectedErrors: []string{
				"api dependency 0: git: repository must not be empty",
			},
		},
		"InvalidAPIDependencyHTTPEmptyURL": {
			input: &Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-project",
				},
				Spec: &ProjectSpec{
					Repository: "xpkg.upbound.io/acmeco/my-project",
					APIDependencies: []APIDependencies{
						{
							Type: "crd",
							HTTP: &APIHTTPReference{
								URL: "",
							},
						},
					},
				},
			},
			expectedErrors: []string{
				"api dependency 0: http: url must not be empty",
			},
		},
		"InvalidAPIDependencyK8sEmptyVersion": {
			input: &Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-project",
				},
				Spec: &ProjectSpec{
					Repository: "xpkg.upbound.io/acmeco/my-project",
					APIDependencies: []APIDependencies{
						{
							Type: "k8s",
							K8s: &APIK8sReference{
								Version: "",
							},
						},
					},
				},
			},
			expectedErrors: []string{
				"api dependency 0: k8s: version must not be empty",
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
						APIs:       "not-default-apis/",
						Functions:  "not-default-functions/",
						Examples:   "not-default-examples/",
						Tests:      "not-default-tests/",
						Operations: "not-default-operations/",
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
						APIs:       "not-default-apis/",
						Functions:  "not-default-functions/",
						Examples:   "not-default-examples/",
						Tests:      "not-default-tests/",
						Operations: "not-default-operations/",
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
						APIs:       "apis",
						Examples:   "examples",
						Functions:  "functions",
						Tests:      "tests",
						Operations: "operations",
					},
					Architectures: []string{"amd64", "arm64"},
					Crossplane: &pkgmetav1.CrossplaneConstraints{
						Version: ">=v2.0.0-rc.0",
					},
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
