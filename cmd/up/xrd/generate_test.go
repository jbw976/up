// Copyright 2025 Upbound Inc.
// All rights reserved

package xrd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	rgdv1alpha1 "github.com/kubernetes-sigs/kro/api/v1alpha1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	v1 "github.com/crossplane/crossplane/v2/apis/apiextensions/v1"
	v2 "github.com/crossplane/crossplane/v2/apis/apiextensions/v2"
)

// TestNewXRDv1 tests the newXRD function.
func TestNewXRDv1(t *testing.T) {
	type want struct {
		xrd *v1.CompositeResourceDefinition
		err error
	}

	cases := map[string]struct {
		inputYAML    string
		customPlural string
		want         want
	}{
		"XRXEKS": {
			inputYAML: `
apiVersion: aws.u5d.io/v1
kind: XEKS
metadata:
  name: test
spec:
  parameters:
    id: test
    region: eu-central-1
`,
			customPlural: "xeks",
			want: want{
				xrd: &v1.CompositeResourceDefinition{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apiextensions.crossplane.io/v1",
						Kind:       "CompositeResourceDefinition",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "xeks.aws.u5d.io",
					},
					Spec: v1.CompositeResourceDefinitionSpec{
						Group: "aws.u5d.io",
						Names: extv1.CustomResourceDefinitionNames{
							Categories: []string{"crossplane"},
							Kind:       "XEKS",
							Plural:     "xeks",
						},
						Versions: []v1.CompositeResourceDefinitionVersion{
							{
								Name:          "v1",
								Referenceable: true,
								Served:        true,
								Schema: &v1.CompositeResourceValidation{
									OpenAPIV3Schema: jsonSchemaPropsToRawExtension(&extv1.JSONSchemaProps{
										Description: "XEKS is the Schema for the XEKS API.",
										Type:        "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"spec": {
												Description: "XEKSSpec defines the desired state of XEKS.",
												Type:        "object",
												Properties: map[string]extv1.JSONSchemaProps{
													"parameters": {
														Type: "object",
														Properties: map[string]extv1.JSONSchemaProps{
															"id": {
																Type: "string",
															},
															"region": {
																Type: "string",
															},
														},
													},
												},
											},
											"status": {
												Description: "XEKSStatus defines the observed state of XEKS.",
												Type:        "object",
											},
										},
										Required: []string{"spec"},
									}),
								},
							},
						},
					},
				},
				err: nil,
			},
		},
		"XRCEKS": {
			inputYAML: `
apiVersion: aws.u5d.io/v1
kind: EKS
metadata:
  name: test
  namespace: test-namespace
spec:
  parameters:
    id: test
    region: eu-central-1
`,
			customPlural: "eks",
			want: want{
				xrd: &v1.CompositeResourceDefinition{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apiextensions.crossplane.io/v1",
						Kind:       "CompositeResourceDefinition",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "xeks.aws.u5d.io",
					},
					Spec: v1.CompositeResourceDefinitionSpec{
						Group: "aws.u5d.io",
						Names: extv1.CustomResourceDefinitionNames{
							Categories: []string{"crossplane"},
							Kind:       "XEKS",
							Plural:     "xeks",
						},
						Versions: []v1.CompositeResourceDefinitionVersion{
							{
								Name:          "v1",
								Referenceable: true,
								Served:        true,
								Schema: &v1.CompositeResourceValidation{
									OpenAPIV3Schema: jsonSchemaPropsToRawExtension(&extv1.JSONSchemaProps{
										Description: "EKS is the Schema for the EKS API.",
										Type:        "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"spec": {
												Description: "EKSSpec defines the desired state of EKS.",
												Type:        "object",
												Properties: map[string]extv1.JSONSchemaProps{
													"parameters": {
														Type: "object",
														Properties: map[string]extv1.JSONSchemaProps{
															"id": {
																Type: "string",
															},
															"region": {
																Type: "string",
															},
														},
													},
												},
											},
											"status": {
												Description: "EKSStatus defines the observed state of EKS.",
												Type:        "object",
											},
										},
										Required: []string{"spec"},
									}),
								},
							},
						},
						ClaimNames: &extv1.CustomResourceDefinitionNames{
							Kind:   "EKS",
							Plural: "eks",
						},
					},
				},
				err: nil,
			},
		},
		"XRPostgres": {
			inputYAML: `
apiVersion: database.u5d.io/v1
kind: Postgres
metadata:
  name: test
spec:
  parameters:
    version: "13"
`,
			customPlural: "Postgreses",
			want: want{
				xrd: &v1.CompositeResourceDefinition{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apiextensions.crossplane.io/v1",
						Kind:       "CompositeResourceDefinition",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "postgreses.database.u5d.io",
					},
					Spec: v1.CompositeResourceDefinitionSpec{
						Group: "database.u5d.io",
						Names: extv1.CustomResourceDefinitionNames{
							Categories: []string{"crossplane"},
							Kind:       "Postgres",
							Plural:     "postgreses",
						},
						Versions: []v1.CompositeResourceDefinitionVersion{
							{
								Name:          "v1",
								Referenceable: true,
								Served:        true,
								Schema: &v1.CompositeResourceValidation{
									OpenAPIV3Schema: jsonSchemaPropsToRawExtension(&extv1.JSONSchemaProps{
										Description: "Postgres is the Schema for the Postgres API.",
										Type:        "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"spec": {
												Description: "PostgresSpec defines the desired state of Postgres.",
												Type:        "object",
												Properties: map[string]extv1.JSONSchemaProps{
													"parameters": {
														Type: "object",
														Properties: map[string]extv1.JSONSchemaProps{
															"version": {
																Type: "string",
															},
														},
													},
												},
											},
											"status": {
												Description: "PostgresStatus defines the observed state of Postgres.",
												Type:        "object",
											},
										},
										Required: []string{"spec"},
									}),
								},
							},
						},
					},
				},
				err: nil,
			},
		},
		"XRBucket": {
			inputYAML: `
apiVersion: storage.u5d.io/v1
kind: Bucket
metadata:
  name: test
spec:
  parameters:
    storage: "13"
`,
			want: want{
				xrd: &v1.CompositeResourceDefinition{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apiextensions.crossplane.io/v1",
						Kind:       "CompositeResourceDefinition",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "buckets.storage.u5d.io",
					},
					Spec: v1.CompositeResourceDefinitionSpec{
						Group: "storage.u5d.io",
						Names: extv1.CustomResourceDefinitionNames{
							Categories: []string{"crossplane"},
							Kind:       "Bucket",
							Plural:     "buckets",
						},
						Versions: []v1.CompositeResourceDefinitionVersion{
							{
								Name:          "v1",
								Referenceable: true,
								Served:        true,
								Schema: &v1.CompositeResourceValidation{
									OpenAPIV3Schema: jsonSchemaPropsToRawExtension(&extv1.JSONSchemaProps{
										Description: "Bucket is the Schema for the Bucket API.",
										Type:        "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"spec": {
												Description: "BucketSpec defines the desired state of Bucket.",
												Type:        "object",
												Properties: map[string]extv1.JSONSchemaProps{
													"parameters": {
														Type: "object",
														Properties: map[string]extv1.JSONSchemaProps{
															"storage": {
																Type: "string",
															},
														},
													},
												},
											},
											"status": {
												Description: "BucketStatus defines the observed state of Bucket.",
												Type:        "object",
											},
										},
										Required: []string{"spec"},
									}),
								},
							},
						},
					},
				},
				err: nil,
			},
		},
		"XRBucketWithStatus": {
			inputYAML: `
apiVersion: storage.u5d.io/v1
kind: Bucket
metadata:
  name: test
spec:
  parameters:
    storage: "13"
status:
  bucketName: test
`,
			want: want{
				xrd: &v1.CompositeResourceDefinition{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apiextensions.crossplane.io/v1",
						Kind:       "CompositeResourceDefinition",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "buckets.storage.u5d.io",
					},
					Spec: v1.CompositeResourceDefinitionSpec{
						Group: "storage.u5d.io",
						Names: extv1.CustomResourceDefinitionNames{
							Categories: []string{"crossplane"},
							Kind:       "Bucket",
							Plural:     "buckets",
						},
						Versions: []v1.CompositeResourceDefinitionVersion{
							{
								Name:          "v1",
								Referenceable: true,
								Served:        true,
								Schema: &v1.CompositeResourceValidation{
									OpenAPIV3Schema: jsonSchemaPropsToRawExtension(&extv1.JSONSchemaProps{
										Description: "Bucket is the Schema for the Bucket API.",
										Type:        "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"spec": {
												Description: "BucketSpec defines the desired state of Bucket.",
												Type:        "object",
												Properties: map[string]extv1.JSONSchemaProps{
													"parameters": {
														Type: "object",
														Properties: map[string]extv1.JSONSchemaProps{
															"storage": {
																Type: "string",
															},
														},
													},
												},
											},
											"status": {
												Description: "BucketStatus defines the observed state of Bucket.",
												Type:        "object",
												Properties: map[string]extv1.JSONSchemaProps{
													"bucketName": {
														Type: "string",
													},
												},
											},
										},
										Required: []string{"spec"},
									}),
								},
							},
						},
					},
				},
				err: nil,
			},
		},
		"MixedTypesInArray": {
			inputYAML: `
apiVersion: aws.u5d.io/v1
kind: MyClaim
metadata:
  name: my-claim
spec:
  parameters:
    - 1
    - "2"
    - true
`,
			customPlural: "myclaims",
			want: want{
				xrd: nil,
				err: errors.New("failed to infer properties for spec: error inferring property for key 'parameters': mixed types detected in array"),
			},
		},
		"NestedMixedTypesInArray": {
			inputYAML: `
apiVersion: aws.u5d.io/v1
kind: MyClaim
metadata:
  name: my-claim
spec:
  parameters:
    chris:
      - 1
      - "2"
      - true
`,
			customPlural: "myclaims",
			want: want{
				xrd: nil,
				err: errors.New("failed to infer properties for spec: error inferring property for key 'parameters': error inferring properties for object: error inferring property for key 'chris': mixed types detected in array"),
			},
		},
		"MissingAPIVersion": {
			inputYAML: `
kind: Postgres
metadata:
  name: test
spec:
  parameters:
    version: "13"
`,
			customPlural: "postgreses",
			want: want{
				xrd: nil,
				err: errors.New("invalid manifest: apiVersion is required"),
			},
		},
		"MissingAPIVersionVersion": {
			inputYAML: `
apiVersion: database.u5d.io
kind: Postgres
metadata:
  name: test
spec:
  parameters:
    version: "13"
`,
			customPlural: "postgreses",
			want: want{
				xrd: nil,
				err: errors.New("invalid manifest: apiVersion must be in the format group/version"),
			},
		},
		"MissingKind": {
			inputYAML: `
apiVersion: database.u5d.io/v1
metadata:
  name: test
spec:
  parameters:
    version: "13"
`,
			customPlural: "postgreses",
			want: want{
				xrd: nil,
				err: errors.New("invalid manifest: kind is required"),
			},
		},
		"MissingMetadataName": {
			inputYAML: `
apiVersion: database.u5d.io/v1
kind: Postgres
spec:
  parameters:
    version: "13"
`,
			customPlural: "postgreses",
			want: want{
				xrd: nil,
				err: errors.New("invalid manifest: metadata.name is required"),
			},
		},
		"MissingSpec": {
			inputYAML: `
apiVersion: database.u5d.io/v1
kind: Postgres
metadata:
  name: test
`,
			customPlural: "postgreses",
			want: want{
				xrd: nil,
				err: errors.New("invalid manifest: spec is required"),
			},
		},
		"InvalidTopLevelKey": {
			inputYAML: `
apiVersion: database.u5d.io/v1
kind: Postgres
metadata:
  name: test
spec:
  parameters:
    version: "13"
invalidKey: shouldNotBeHere
`,
			customPlural: "postgreses",
			want: want{
				xrd: nil,
				err: errors.New("invalid manifest: only apiVersion, kind, metadata, spec, and status are allowed as top-level keys"),
			},
		},
		"InvalidAPIVersionMultipleSlashes": {
			inputYAML: `
apiVersion: invalid/group/version/v1
kind: InvalidResource
metadata:
  name: test
spec:
  parameters:
    key: value
`,
			customPlural: "invalidresources",
			want: want{
				xrd: nil,
				err: errors.New("invalid manifest: apiVersion must be in the format group/version"),
			},
		},
		"RemoveXPStandardFieldsFromSpec": {
			inputYAML: `
apiVersion: aws.u5d.io/v1
kind: XEKS
metadata:
  name: test
spec:
  parameters:
    id: test
    region: eu-central-1
  resourceRefs:
    - name: resource1
  writeConnectionSecretToRef:
    name: secret
  publishConnectionDetailsTo:
    name: details
  environmentConfigRefs:
    - name: config1
  compositionSelector:
    matchLabels:
      layer: functions
`,
			customPlural: "xeks",
			want: want{
				xrd: &v1.CompositeResourceDefinition{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apiextensions.crossplane.io/v1",
						Kind:       "CompositeResourceDefinition",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "xeks.aws.u5d.io",
					},
					Spec: v1.CompositeResourceDefinitionSpec{
						Group: "aws.u5d.io",
						Names: extv1.CustomResourceDefinitionNames{
							Categories: []string{"crossplane"},
							Kind:       "XEKS",
							Plural:     "xeks",
						},
						Versions: []v1.CompositeResourceDefinitionVersion{
							{
								Name:          "v1",
								Referenceable: true,
								Served:        true,
								Schema: &v1.CompositeResourceValidation{
									OpenAPIV3Schema: jsonSchemaPropsToRawExtension(&extv1.JSONSchemaProps{
										Description: "XEKS is the Schema for the XEKS API.",
										Type:        "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"spec": {
												Description: "XEKSSpec defines the desired state of XEKS.",
												Type:        "object",
												Properties: map[string]extv1.JSONSchemaProps{
													"parameters": {
														Type: "object",
														Properties: map[string]extv1.JSONSchemaProps{
															"id": {
																Type: "string",
															},
															"region": {
																Type: "string",
															},
														},
													},
												},
											},
											"status": {
												Description: "XEKSStatus defines the observed state of XEKS.",
												Type:        "object",
											},
										},
										Required: []string{"spec"},
									}),
								},
							},
						},
					},
				},
				err: nil,
			},
		},
		"RemoveOtherXPStandardFieldsFromSpec": {
			inputYAML: `
apiVersion: aws.u5d.io/v1
kind: XEKS
metadata:
  name: test
spec:
  parameters:
    id: test
    region: eu-central-1
  compositionRef:
    name: test-composition
  compositionRevisionRef:
    name: test-revision
  claimRef:
    name: test-claim
`,
			customPlural: "xeks",
			want: want{
				xrd: &v1.CompositeResourceDefinition{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apiextensions.crossplane.io/v1",
						Kind:       "CompositeResourceDefinition",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "xeks.aws.u5d.io",
					},
					Spec: v1.CompositeResourceDefinitionSpec{
						Group: "aws.u5d.io",
						Names: extv1.CustomResourceDefinitionNames{
							Categories: []string{"crossplane"},
							Kind:       "XEKS",
							Plural:     "xeks",
						},
						Versions: []v1.CompositeResourceDefinitionVersion{
							{
								Name:          "v1",
								Referenceable: true,
								Served:        true,
								Schema: &v1.CompositeResourceValidation{
									OpenAPIV3Schema: jsonSchemaPropsToRawExtension(&extv1.JSONSchemaProps{
										Description: "XEKS is the Schema for the XEKS API.",
										Type:        "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"spec": {
												Description: "XEKSSpec defines the desired state of XEKS.",
												Type:        "object",
												Properties: map[string]extv1.JSONSchemaProps{
													"parameters": {
														Type: "object",
														Properties: map[string]extv1.JSONSchemaProps{
															"id": {
																Type: "string",
															},
															"region": {
																Type: "string",
															},
														},
													},
												},
											},
											"status": {
												Description: "XEKSStatus defines the observed state of XEKS.",
												Type:        "object",
											},
										},
										Required: []string{"spec"},
									}),
								},
							},
						},
					},
				},
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := newXRDv1([]byte(tc.inputYAML), tc.customPlural)

			// Compare error messages as strings, trimming whitespace for safety
			gotErrMsg := ""
			wantErrMsg := ""

			if err != nil {
				gotErrMsg = strings.TrimSpace(err.Error())
			}
			if tc.want.err != nil {
				wantErrMsg = strings.TrimSpace(tc.want.err.Error())
			}

			if gotErrMsg != wantErrMsg {
				t.Errorf("newXRD() error - got: %q, want: %q", gotErrMsg, wantErrMsg)
			}

			// Compare the output XRD (ignoring "Required" field for simplicity)
			if diff := cmp.Diff(got, tc.want.xrd, cmpopts.IgnoreFields(extv1.JSONSchemaProps{}, "Required")); diff != "" {
				t.Errorf("newXRD() -got, +want:\n%s", diff)
			}
		})
	}
}

// TestNewXRDv2 tests the newXRDv2 function.
func TestNewXRDv2(t *testing.T) {
	type want struct {
		xrd *v2.CompositeResourceDefinition
		err error
	}

	cases := map[string]struct {
		inputYAML    string
		customPlural string
		want         want
	}{
		"ClusterScopedXR": {
			inputYAML: `
apiVersion: aws.u5d.io/v1
kind: XEKS
metadata:
  name: test
spec:
  parameters:
    id: test
    region: eu-central-1
`,
			customPlural: "xeks",
			want: want{
				xrd: &v2.CompositeResourceDefinition{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apiextensions.crossplane.io/v2",
						Kind:       "CompositeResourceDefinition",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "xeks.aws.u5d.io",
					},
					Spec: v2.CompositeResourceDefinitionSpec{
						Group: "aws.u5d.io",
						Scope: v2.CompositeResourceScopeCluster,
						Names: extv1.CustomResourceDefinitionNames{
							Categories: []string{"crossplane"},
							Kind:       "XEKS",
							Plural:     "xeks",
						},
						Versions: []v2.CompositeResourceDefinitionVersion{
							{
								Name:          "v1",
								Referenceable: true,
								Served:        true,
								Schema: &v2.CompositeResourceValidation{
									OpenAPIV3Schema: jsonSchemaPropsToRawExtension(&extv1.JSONSchemaProps{
										Description: "XEKS is the Schema for the XEKS API.",
										Type:        "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"spec": {
												Description: "XEKSSpec defines the desired state of XEKS.",
												Type:        "object",
												Properties: map[string]extv1.JSONSchemaProps{
													"parameters": {
														Type: "object",
														Properties: map[string]extv1.JSONSchemaProps{
															"id": {
																Type: "string",
															},
															"region": {
																Type: "string",
															},
														},
													},
												},
											},
											"status": {
												Description: "XEKSStatus defines the observed state of XEKS.",
												Type:        "object",
											},
										},
										Required: []string{"spec"},
									}),
								},
							},
						},
					},
				},
				err: nil,
			},
		},
		"NamespaceScopedXRC": {
			inputYAML: `
apiVersion: aws.u5d.io/v1
kind: EKS
metadata:
  name: test
  namespace: test-namespace
spec:
  parameters:
    id: test
    region: eu-central-1
`,
			customPlural: "eks",
			want: want{
				xrd: &v2.CompositeResourceDefinition{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apiextensions.crossplane.io/v2",
						Kind:       "CompositeResourceDefinition",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "eks.aws.u5d.io",
					},
					Spec: v2.CompositeResourceDefinitionSpec{
						Group: "aws.u5d.io",
						Scope: v2.CompositeResourceScopeNamespaced,
						Names: extv1.CustomResourceDefinitionNames{
							Categories: []string{"crossplane"},
							Kind:       "EKS",
							Plural:     "eks",
						},
						Versions: []v2.CompositeResourceDefinitionVersion{
							{
								Name:          "v1",
								Referenceable: true,
								Served:        true,
								Schema: &v2.CompositeResourceValidation{
									OpenAPIV3Schema: jsonSchemaPropsToRawExtension(&extv1.JSONSchemaProps{
										Description: "EKS is the Schema for the EKS API.",
										Type:        "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"spec": {
												Description: "EKSSpec defines the desired state of EKS.",
												Type:        "object",
												Properties: map[string]extv1.JSONSchemaProps{
													"parameters": {
														Type: "object",
														Properties: map[string]extv1.JSONSchemaProps{
															"id": {
																Type: "string",
															},
															"region": {
																Type: "string",
															},
														},
													},
												},
											},
											"status": {
												Description: "EKSStatus defines the observed state of EKS.",
												Type:        "object",
											},
										},
										Required: []string{"spec"},
									}),
								},
							},
						},
					},
				},
				err: nil,
			},
		},
		"CustomPluralPostgres": {
			inputYAML: `
apiVersion: database.u5d.io/v1
kind: Postgres
metadata:
  name: test
spec:
  parameters:
    version: "13"
`,
			customPlural: "postgreses",
			want: want{
				xrd: &v2.CompositeResourceDefinition{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apiextensions.crossplane.io/v2",
						Kind:       "CompositeResourceDefinition",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "postgreses.database.u5d.io",
					},
					Spec: v2.CompositeResourceDefinitionSpec{
						Group: "database.u5d.io",
						Scope: v2.CompositeResourceScopeCluster,
						Names: extv1.CustomResourceDefinitionNames{
							Categories: []string{"crossplane"},
							Kind:       "Postgres",
							Plural:     "postgreses",
						},
						Versions: []v2.CompositeResourceDefinitionVersion{
							{
								Name:          "v1",
								Referenceable: true,
								Served:        true,
								Schema: &v2.CompositeResourceValidation{
									OpenAPIV3Schema: jsonSchemaPropsToRawExtension(&extv1.JSONSchemaProps{
										Description: "Postgres is the Schema for the Postgres API.",
										Type:        "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"spec": {
												Description: "PostgresSpec defines the desired state of Postgres.",
												Type:        "object",
												Properties: map[string]extv1.JSONSchemaProps{
													"parameters": {
														Type: "object",
														Properties: map[string]extv1.JSONSchemaProps{
															"version": {
																Type: "string",
															},
														},
													},
												},
											},
											"status": {
												Description: "PostgresStatus defines the observed state of Postgres.",
												Type:        "object",
											},
										},
										Required: []string{"spec"},
									}),
								},
							},
						},
					},
				},
				err: nil,
			},
		},
		"BucketWithStatus": {
			inputYAML: `
apiVersion: storage.u5d.io/v1
kind: Bucket
metadata:
  name: test
spec:
  parameters:
    storage: "13"
status:
  bucketName: test
`,
			want: want{
				xrd: &v2.CompositeResourceDefinition{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apiextensions.crossplane.io/v2",
						Kind:       "CompositeResourceDefinition",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "buckets.storage.u5d.io",
					},
					Spec: v2.CompositeResourceDefinitionSpec{
						Group: "storage.u5d.io",
						Scope: v2.CompositeResourceScopeCluster,
						Names: extv1.CustomResourceDefinitionNames{
							Categories: []string{"crossplane"},
							Kind:       "Bucket",
							Plural:     "buckets",
						},
						Versions: []v2.CompositeResourceDefinitionVersion{
							{
								Name:          "v1",
								Referenceable: true,
								Served:        true,
								Schema: &v2.CompositeResourceValidation{
									OpenAPIV3Schema: jsonSchemaPropsToRawExtension(&extv1.JSONSchemaProps{
										Description: "Bucket is the Schema for the Bucket API.",
										Type:        "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"spec": {
												Description: "BucketSpec defines the desired state of Bucket.",
												Type:        "object",
												Properties: map[string]extv1.JSONSchemaProps{
													"parameters": {
														Type: "object",
														Properties: map[string]extv1.JSONSchemaProps{
															"storage": {
																Type: "string",
															},
														},
													},
												},
											},
											"status": {
												Description: "BucketStatus defines the observed state of Bucket.",
												Type:        "object",
												Properties: map[string]extv1.JSONSchemaProps{
													"bucketName": {
														Type: "string",
													},
												},
											},
										},
										Required: []string{"spec"},
									}),
								},
							},
						},
					},
				},
				err: nil,
			},
		},
		"RemoveXPStandardFieldsFromSpec": {
			inputYAML: `
apiVersion: aws.u5d.io/v1
kind: XEKS
metadata:
  name: test
spec:
  parameters:
    id: test
    region: eu-central-1
  resourceRefs:
    - name: resource1
  writeConnectionSecretToRef:
    name: secret
  publishConnectionDetailsTo:
    name: details
  environmentConfigRefs:
    - name: config1
  compositionSelector:
    matchLabels:
      layer: functions
`,
			customPlural: "xeks",
			want: want{
				xrd: &v2.CompositeResourceDefinition{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apiextensions.crossplane.io/v2",
						Kind:       "CompositeResourceDefinition",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "xeks.aws.u5d.io",
					},
					Spec: v2.CompositeResourceDefinitionSpec{
						Group: "aws.u5d.io",
						Scope: v2.CompositeResourceScopeCluster,
						Names: extv1.CustomResourceDefinitionNames{
							Categories: []string{"crossplane"},
							Kind:       "XEKS",
							Plural:     "xeks",
						},
						Versions: []v2.CompositeResourceDefinitionVersion{
							{
								Name:          "v1",
								Referenceable: true,
								Served:        true,
								Schema: &v2.CompositeResourceValidation{
									OpenAPIV3Schema: jsonSchemaPropsToRawExtension(&extv1.JSONSchemaProps{
										Description: "XEKS is the Schema for the XEKS API.",
										Type:        "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"spec": {
												Description: "XEKSSpec defines the desired state of XEKS.",
												Type:        "object",
												Properties: map[string]extv1.JSONSchemaProps{
													"parameters": {
														Type: "object",
														Properties: map[string]extv1.JSONSchemaProps{
															"id": {
																Type: "string",
															},
															"region": {
																Type: "string",
															},
														},
													},
												},
											},
											"status": {
												Description: "XEKSStatus defines the observed state of XEKS.",
												Type:        "object",
											},
										},
										Required: []string{"spec"},
									}),
								},
							},
						},
					},
				},
				err: nil,
			},
		},
		"MissingAPIVersion": {
			inputYAML: `
kind: Postgres
metadata:
  name: test
spec:
  parameters:
    version: "13"
`,
			customPlural: "postgreses",
			want: want{
				xrd: nil,
				err: errors.New("invalid manifest: apiVersion is required"),
			},
		},
		"MissingKind": {
			inputYAML: `
apiVersion: database.u5d.io/v1
metadata:
  name: test
spec:
  parameters:
    version: "13"
`,
			customPlural: "postgreses",
			want: want{
				xrd: nil,
				err: errors.New("invalid manifest: kind is required"),
			},
		},
		"InvalidTopLevelKey": {
			inputYAML: `
apiVersion: database.u5d.io/v1
kind: Postgres
metadata:
  name: test
spec:
  parameters:
    version: "13"
invalidKey: shouldNotBeHere
`,
			customPlural: "postgreses",
			want: want{
				xrd: nil,
				err: errors.New("invalid manifest: only apiVersion, kind, metadata, spec, and status are allowed as top-level keys"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := newXRDv2([]byte(tc.inputYAML), tc.customPlural)

			// Compare error messages as strings, trimming whitespace for safety
			gotErrMsg := ""
			wantErrMsg := ""

			if err != nil {
				gotErrMsg = strings.TrimSpace(err.Error())
			}
			if tc.want.err != nil {
				wantErrMsg = strings.TrimSpace(tc.want.err.Error())
			}

			if gotErrMsg != wantErrMsg {
				t.Errorf("newXRDv2() error - got: %q, want: %q", gotErrMsg, wantErrMsg)
			}

			if diff := cmp.Diff(got, tc.want.xrd, cmpopts.IgnoreFields(extv1.JSONSchemaProps{}, "Required")); diff != "" {
				t.Errorf("newXRDv2() -got, +want:\n%s", diff)
			}
		})
	}
}

// TestNewXRDFromSimpleSchema tests creating XRDs from SimpleSchema format.
func TestNewXRDFromSimpleSchema(t *testing.T) {
	type want struct {
		xrd *v2.CompositeResourceDefinition
		err error
	}

	cases := map[string]struct {
		inputYAML    string
		customPlural string
		want         want
	}{
		"SimpleSchemaApplication": {
			inputYAML: `apiVersion: chris.io/v1alpha1
kind: Application
metadata:
  name: default
spec:
  name: string
  image: string | default="nginx"
  ingress:
    enabled: boolean | default=false
status:
  deploymentConditions: ${deployment.status.conditions}
  availableReplicas: ${deployment.status.availableReplicas}`,
			want: want{
				xrd: &v2.CompositeResourceDefinition{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apiextensions.crossplane.io/v2",
						Kind:       "CompositeResourceDefinition",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "applications.chris.io",
					},
					Spec: v2.CompositeResourceDefinitionSpec{
						Group: "chris.io",
						Scope: v2.CompositeResourceScopeNamespaced,
						Names: extv1.CustomResourceDefinitionNames{
							Categories: []string{"crossplane"},
							Kind:       "Application",
							Plural:     "applications",
						},
						Versions: []v2.CompositeResourceDefinitionVersion{
							{
								Name:          "v1alpha1",
								Referenceable: true,
								Served:        true,
								Schema: &v2.CompositeResourceValidation{
									OpenAPIV3Schema: jsonSchemaPropsToRawExtension(&extv1.JSONSchemaProps{
										Description: "Application is the Schema for the Application API.",
										Type:        "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"spec": {
												Type:    "object",
												Default: &extv1.JSON{Raw: []byte(`{}`)},
												Properties: map[string]extv1.JSONSchemaProps{
													"name": {
														Type: "string",
													},
													"image": {
														Type:    "string",
														Default: &extv1.JSON{Raw: []byte(`"nginx"`)},
													},
													"ingress": {
														Type:    "object",
														Default: &extv1.JSON{Raw: []byte(`{}`)},
														Properties: map[string]extv1.JSONSchemaProps{
															"enabled": {
																Type:    "boolean",
																Default: &extv1.JSON{Raw: []byte(`false`)},
															},
														},
													},
												},
											},
											"status": {
												Type: "object",
												Properties: map[string]extv1.JSONSchemaProps{
													"deploymentConditions": {
														XPreserveUnknownFields: &[]bool{true}[0],
													},
													"availableReplicas": {
														XPreserveUnknownFields: &[]bool{true}[0],
													},
												},
											},
										},
										Required: []string{"spec"},
									}),
								},
							},
						},
					},
				},
				err: nil,
			},
		},
		"SimpleSchemaWithCustomPlural": {
			inputYAML: `apiVersion: example.com/v1
kind: Database
metadata:
  name: test-db
spec:
  engine: string
  version: string`,
			customPlural: "databases",
			want: want{
				xrd: &v2.CompositeResourceDefinition{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apiextensions.crossplane.io/v2",
						Kind:       "CompositeResourceDefinition",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "databases.example.com",
					},
					Spec: v2.CompositeResourceDefinitionSpec{
						Group: "example.com",
						Scope: v2.CompositeResourceScopeNamespaced,
						Names: extv1.CustomResourceDefinitionNames{
							Categories: []string{"crossplane"},
							Kind:       "Database",
							Plural:     "databases",
						},
						Versions: []v2.CompositeResourceDefinitionVersion{
							{
								Name:          "v1",
								Referenceable: true,
								Served:        true,
								Schema: &v2.CompositeResourceValidation{
									OpenAPIV3Schema: jsonSchemaPropsToRawExtension(&extv1.JSONSchemaProps{
										Description: "Database is the Schema for the Database API.",
										Type:        "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"spec": {
												Type: "object",
												Properties: map[string]extv1.JSONSchemaProps{
													"engine": {
														Type: "string",
													},
													"version": {
														Type: "string",
													},
												},
											},
											"status": {
												Type:       "object",
												Properties: map[string]extv1.JSONSchemaProps{},
											},
										},
										Required: []string{"spec"},
									}),
								},
							},
						},
					},
				},
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := fromSimpleSchema([]byte(tc.inputYAML), tc.customPlural)

			gotErrMsg := ""
			wantErrMsg := ""

			if err != nil {
				gotErrMsg = strings.TrimSpace(err.Error())
			}
			if tc.want.err != nil {
				wantErrMsg = strings.TrimSpace(tc.want.err.Error())
			}

			if gotErrMsg != wantErrMsg {
				t.Errorf("newXRDFromSimpleSchema() error - got: %q, want: %q", gotErrMsg, wantErrMsg)
			}

			if diff := cmp.Diff(got, tc.want.xrd, cmpopts.IgnoreFields(extv1.JSONSchemaProps{}, "Required")); diff != "" {
				t.Errorf("newXRDFromSimpleSchema() -got, +want:\n%s", diff)
			}
		})
	}
}

// TestXRDFromResourceGraphDefinition tests creating XRDs from ResourceGraphDefinition format.
func TestXRDFromResourceGraphDefinition(t *testing.T) {
	type want struct {
		xrd *v2.CompositeResourceDefinition
		err error
	}

	cases := map[string]struct {
		inputYAML    string
		customPlural string
		want         want
	}{
		"ResourceGraphDefinitionApplication": {
			inputYAML: `apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: application
spec:
  schema:
    apiVersion: v1alpha1
    kind: Application
    spec:
      name: string
      image: string | default="nginx"
      ingress:
        enabled: boolean | default=false
    status:
      deploymentConditions: ${deployment.status.conditions}
      availableReplicas: ${deployment.status.availableReplicas}
    additionalPrinterColumns:
      - jsonPath: .status.availableReplicas
        name: Available replicas
        type: integer
      - jsonPath: .spec.image
        name: Image
        type: string
  resources:
    - id: deployment
      template:
        apiVersion: apps/v1
        kind: Deployment`,
			want: want{
				xrd: &v2.CompositeResourceDefinition{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apiextensions.crossplane.io/v2",
						Kind:       "CompositeResourceDefinition",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "applications.kro.run",
					},
					Spec: v2.CompositeResourceDefinitionSpec{
						Group: "kro.run",
						Scope: v2.CompositeResourceScopeNamespaced,
						Names: extv1.CustomResourceDefinitionNames{
							Categories: []string{"crossplane"},
							Kind:       "Application",
							Plural:     "applications",
						},
						Versions: []v2.CompositeResourceDefinitionVersion{
							{
								Name:          "v1alpha1",
								Referenceable: true,
								Served:        true,
								AdditionalPrinterColumns: []extv1.CustomResourceColumnDefinition{
									{
										JSONPath: ".status.availableReplicas",
										Name:     "Available replicas",
										Type:     "integer",
									},
									{
										JSONPath: ".spec.image",
										Name:     "Image",
										Type:     "string",
									},
								},
								Schema: &v2.CompositeResourceValidation{
									OpenAPIV3Schema: jsonSchemaPropsToRawExtension(&extv1.JSONSchemaProps{
										Description: "Application is the Schema for the Application API.",
										Type:        "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"spec": {
												Type:    "object",
												Default: &extv1.JSON{Raw: []byte(`{}`)},
												Properties: map[string]extv1.JSONSchemaProps{
													"name": {
														Type: "string",
													},
													"image": {
														Type:    "string",
														Default: &extv1.JSON{Raw: []byte(`"nginx"`)},
													},
													"ingress": {
														Type:    "object",
														Default: &extv1.JSON{Raw: []byte(`{}`)},
														Properties: map[string]extv1.JSONSchemaProps{
															"enabled": {
																Type:    "boolean",
																Default: &extv1.JSON{Raw: []byte(`false`)},
															},
														},
													},
												},
											},
											"status": {
												Type: "object",
												Properties: map[string]extv1.JSONSchemaProps{
													"deploymentConditions": {
														XPreserveUnknownFields: &[]bool{true}[0],
													},
													"availableReplicas": {
														XPreserveUnknownFields: &[]bool{true}[0],
													},
												},
											},
										},
										Required: []string{"spec"},
									}),
								},
							},
						},
					},
				},
				err: nil,
			},
		},
		"ResourceGraphDefinitionWithFullAPIVersion": {
			inputYAML: `apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: myapp
spec:
  schema:
    apiVersion: custom.io/v1beta1
    kind: MyApp
    spec:
      replicas: integer
    status:
      ready: ${deployment.status.ready}`,
			want: want{
				xrd: &v2.CompositeResourceDefinition{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apiextensions.crossplane.io/v2",
						Kind:       "CompositeResourceDefinition",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "myapps.custom.io",
					},
					Spec: v2.CompositeResourceDefinitionSpec{
						Group: "custom.io",
						Scope: v2.CompositeResourceScopeNamespaced,
						Names: extv1.CustomResourceDefinitionNames{
							Categories: []string{"crossplane"},
							Kind:       "MyApp",
							Plural:     "myapps",
						},
						Versions: []v2.CompositeResourceDefinitionVersion{
							{
								Name:          "v1beta1",
								Referenceable: true,
								Served:        true,
								Schema: &v2.CompositeResourceValidation{
									OpenAPIV3Schema: jsonSchemaPropsToRawExtension(&extv1.JSONSchemaProps{
										Description: "MyApp is the Schema for the MyApp API.",
										Type:        "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"spec": {
												Type: "object",
												Properties: map[string]extv1.JSONSchemaProps{
													"replicas": {
														Type: "integer",
													},
												},
											},
											"status": {
												Type: "object",
												Properties: map[string]extv1.JSONSchemaProps{
													"ready": {
														XPreserveUnknownFields: &[]bool{true}[0],
													},
												},
											},
										},
										Required: []string{"spec"},
									}),
								},
							},
						},
					},
				},
				err: nil,
			},
		},
		"ResourceGraphDefinitionWithCustomPlural": {
			inputYAML: `apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: database
spec:
  schema:
    apiVersion: v1
    kind: Database
    spec:
      engine: string`,
			customPlural: "databases",
			want: want{
				xrd: &v2.CompositeResourceDefinition{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apiextensions.crossplane.io/v2",
						Kind:       "CompositeResourceDefinition",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "databases.kro.run",
					},
					Spec: v2.CompositeResourceDefinitionSpec{
						Group: "kro.run",
						Scope: v2.CompositeResourceScopeNamespaced,
						Names: extv1.CustomResourceDefinitionNames{
							Categories: []string{"crossplane"},
							Kind:       "Database",
							Plural:     "databases",
						},
						Versions: []v2.CompositeResourceDefinitionVersion{
							{
								Name:          "v1",
								Referenceable: true,
								Served:        true,
								Schema: &v2.CompositeResourceValidation{
									OpenAPIV3Schema: jsonSchemaPropsToRawExtension(&extv1.JSONSchemaProps{
										Description: "Database is the Schema for the Database API.",
										Type:        "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"spec": {
												Type: "object",
												Properties: map[string]extv1.JSONSchemaProps{
													"engine": {
														Type: "string",
													},
												},
											},
											"status": {
												Type:       "object",
												Properties: map[string]extv1.JSONSchemaProps{},
											},
										},
										Required: []string{"spec"},
									}),
								},
							},
						},
					},
				},
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var rgd rgdv1alpha1.ResourceGraphDefinition
			if err := yaml.Unmarshal([]byte(tc.inputYAML), &rgd); err != nil {
				t.Fatalf("Failed to unmarshal ResourceGraphDefinition: %v", err)
			}

			got, err := fromResourceGraphDefinition(&rgd, tc.customPlural)

			gotErrMsg := ""
			wantErrMsg := ""

			if err != nil {
				gotErrMsg = strings.TrimSpace(err.Error())
			}
			if tc.want.err != nil {
				wantErrMsg = strings.TrimSpace(tc.want.err.Error())
			}

			if gotErrMsg != wantErrMsg {
				t.Errorf("xrdFromResourceGraphDefinition() error - got: %q, want: %q", gotErrMsg, wantErrMsg)
			}

			if diff := cmp.Diff(got, tc.want.xrd, cmpopts.IgnoreFields(extv1.JSONSchemaProps{}, "Required")); diff != "" {
				t.Errorf("xrdFromResourceGraphDefinition() -got, +want:\n%s", diff)
			}
		})
	}
}

// helper function to convert JSONSchemaProps to RawExtension.
func jsonSchemaPropsToRawExtension(schema *extv1.JSONSchemaProps) runtime.RawExtension {
	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		panic(err) // This should never happen in tests with valid schema
	}
	return runtime.RawExtension{Raw: schemaBytes}
}
