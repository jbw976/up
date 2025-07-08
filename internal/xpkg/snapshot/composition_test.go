// Copyright 2025 Upbound Inc.
// All rights reserved

package snapshot

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/afero"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kube-openapi/pkg/validation/validate"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"

	mxpkg "github.com/upbound/up/internal/xpkg/dep/marshaler/xpkg"
	"github.com/upbound/up/internal/xpkg/scheme"
	"github.com/upbound/up/internal/xpkg/snapshot/validator"
	"github.com/upbound/up/internal/xpkg/workspace"

	_ "embed"
)

//go:embed testdata/upbound.yaml
var projectFile []byte

func TestCompositionValidationPipeline(t *testing.T) {
	objScheme, _ := scheme.BuildObjectScheme()
	metaScheme, _ := scheme.BuildMetaScheme()
	ctx := t.Context()

	s := &Snapshot{
		objScheme:  objScheme,
		metaScheme: metaScheme,
		log:        logging.NewNopLogger(),
	}

	type args struct {
		workspace  *workspace.Workspace
		data       runtime.Object
		validators map[schema.GroupVersionKind]validator.Validator
		packages   map[string]*mxpkg.ParsedPackage
	}
	type want struct {
		result *validate.Result
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ValidPipeline": {
			reason: "Validator should not return errors when a pipeline is valid.",
			args: args{
				workspace: func() *workspace.Workspace {
					f := afero.NewMemMapFs()
					_ = afero.WriteFile(f, "/upbound.yaml", projectFile, 0o644)
					_ = f.MkdirAll("/functions/my-function", 0o755)
					ws, _ := workspace.New("/", workspace.WithFS(f))
					return ws
				}(),
				data: &v1.Composition{
					TypeMeta: apimetav1.TypeMeta{
						Kind:       v1.CompositionKind,
						APIVersion: v1.SchemeGroupVersion.String(),
					},
					Spec: v1.CompositionSpec{
						Pipeline: []v1.PipelineStep{
							// Embedded function step.
							{
								Step: "my-function",
								FunctionRef: v1.FunctionReference{
									Name: "upbound-project-getting-startedmy-function",
								},
							},
							// Normal function step.
							{
								Step: "auto-ready",
								FunctionRef: v1.FunctionReference{
									Name: "crossplane-contrib-function-auto-ready",
								},
							},
						},
					},
				},
			},
			want: want{
				result: &validate.Result{},
			},
		},
		"FunctionRefMissingDependency": {
			reason: "Pipeline functions should refer to a package dependency.",
			args: args{
				workspace: func() *workspace.Workspace {
					f := afero.NewMemMapFs()
					_ = afero.WriteFile(f, "/upbound.yaml", projectFile, 0o644)
					ws, _ := workspace.New("/", workspace.WithFS(f))
					return ws
				}(),
				data: &v1.Composition{
					TypeMeta: apimetav1.TypeMeta{
						Kind:       v1.CompositionKind,
						APIVersion: v1.SchemeGroupVersion.String(),
					},
					Spec: v1.CompositionSpec{
						Pipeline: []v1.PipelineStep{{
							Step: "invalid",
							FunctionRef: v1.FunctionReference{
								Name: "acme-co-custom-function",
							},
						}},
					},
				},
			},
			want: want{
				result: &validate.Result{
					Errors: []error{
						&validator.ValidationError{
							TypeCode: validator.WarningTypeCode,
							Message:  `package does not depend on function "acme-co-custom-function"`,
							Name:     "spec.pipeline[0].functionRef.name",
						},
					},
				},
			},
		},
		"FunctionRefInputToFunctionWithNoInput": {
			reason: "Providing input to a function with no input type is an error.",
			args: args{
				workspace: func() *workspace.Workspace {
					f := afero.NewMemMapFs()
					_ = afero.WriteFile(f, "/upbound.yaml", projectFile, 0o644)
					ws, _ := workspace.New("/", workspace.WithFS(f))
					return ws
				}(),
				packages: map[string]*mxpkg.ParsedPackage{
					"xpkg.upbound.io/crossplane-contrib/function-auto-ready": {
						PType: v1beta1.FunctionPackageType,
						Objs:  []runtime.Object{},
					},
				},
				data: &v1.Composition{
					TypeMeta: apimetav1.TypeMeta{
						Kind:       v1.CompositionKind,
						APIVersion: v1.SchemeGroupVersion.String(),
					},
					Spec: v1.CompositionSpec{
						Pipeline: []v1.PipelineStep{{
							Step: "auto-ready",
							FunctionRef: v1.FunctionReference{
								Name: "crossplane-contrib-function-auto-ready",
							},
							Input: &runtime.RawExtension{
								Raw: []byte(`{"apiVersion": "v1"}`),
							},
						}},
					},
				},
			},
			want: want{
				result: &validate.Result{
					Errors: []error{
						&validator.ValidationError{
							TypeCode: 100,
							Message:  `function "crossplane-contrib-function-auto-ready" does not take input`,
							Name:     "spec.pipeline[0].input",
						},
					},
				},
			},
		},
		"FunctionRefUnparsableInput": {
			reason: "Providing malformed input to a function is an error.",
			args: args{
				workspace: func() *workspace.Workspace {
					f := afero.NewMemMapFs()
					_ = afero.WriteFile(f, "/upbound.yaml", projectFile, 0o644)
					ws, _ := workspace.New("/", workspace.WithFS(f))
					return ws
				}(),
				packages: map[string]*mxpkg.ParsedPackage{
					"xpkg.upbound.io/crossplane-contrib/function-auto-ready": {
						PType: v1beta1.FunctionPackageType,
						Objs: []runtime.Object{
							&apiextv1.CustomResourceDefinition{
								TypeMeta: apimetav1.TypeMeta{
									APIVersion: apiextv1.SchemeGroupVersion.String(),
									Kind:       "CustomResourceDefinition",
								},
								ObjectMeta: apimetav1.ObjectMeta{
									Name: "input.my-function.com",
								},
								Spec: apiextv1.CustomResourceDefinitionSpec{
									Group: "my-function.com",
									Names: apiextv1.CustomResourceDefinitionNames{
										Plural:   "inputs",
										Singular: "input",
										Kind:     "Input",
										ListKind: "InputList",
									},
									Versions: []apiextv1.CustomResourceDefinitionVersion{{
										Name:    "v1alpha1",
										Served:  true,
										Storage: true,
										Schema: &apiextv1.CustomResourceValidation{
											OpenAPIV3Schema: &apiextv1.JSONSchemaProps{},
										},
									}},
								},
							},
						},
					},
				},
				data: &v1.Composition{
					TypeMeta: apimetav1.TypeMeta{
						Kind:       v1.CompositionKind,
						APIVersion: v1.SchemeGroupVersion.String(),
					},
					Spec: v1.CompositionSpec{
						Pipeline: []v1.PipelineStep{{
							Step: "auto-ready",
							FunctionRef: v1.FunctionReference{
								Name: "crossplane-contrib-function-auto-ready",
							},
							Input: &runtime.RawExtension{
								Raw: []byte(`{"apiVersion": "v1"}`),
							},
						}},
					},
				},
			},
			want: want{
				result: &validate.Result{
					Errors: []error{
						&validator.ValidationError{
							TypeCode: 100,
							Message:  `Object 'Kind' is missing in '{"apiVersion":"v1"}'`,
							Name:     "spec.pipeline[0].input",
						},
					},
				},
			},
		},
		"FunctionRefWrongInputKind": {
			reason: "Providing the wrong kind of input to a function is an error.",
			args: args{
				workspace: func() *workspace.Workspace {
					f := afero.NewMemMapFs()
					_ = afero.WriteFile(f, "/upbound.yaml", projectFile, 0o644)
					ws, _ := workspace.New("/", workspace.WithFS(f))
					return ws
				}(),
				packages: map[string]*mxpkg.ParsedPackage{
					"xpkg.upbound.io/crossplane-contrib/function-auto-ready": {
						PType: v1beta1.FunctionPackageType,
						Objs: []runtime.Object{
							&apiextv1.CustomResourceDefinition{
								TypeMeta: apimetav1.TypeMeta{
									APIVersion: apiextv1.SchemeGroupVersion.String(),
									Kind:       "CustomResourceDefinition",
								},
								ObjectMeta: apimetav1.ObjectMeta{
									Name: "input.my-function.com",
								},
								Spec: apiextv1.CustomResourceDefinitionSpec{
									Group: "my-function.com",
									Names: apiextv1.CustomResourceDefinitionNames{
										Plural:   "inputs",
										Singular: "input",
										Kind:     "Input",
										ListKind: "InputList",
									},
									Versions: []apiextv1.CustomResourceDefinitionVersion{{
										Name:    "v1alpha1",
										Served:  true,
										Storage: true,
										Schema: &apiextv1.CustomResourceValidation{
											OpenAPIV3Schema: &apiextv1.JSONSchemaProps{},
										},
									}},
								},
							},
						},
					},
				},
				data: &v1.Composition{
					TypeMeta: apimetav1.TypeMeta{
						Kind:       v1.CompositionKind,
						APIVersion: v1.SchemeGroupVersion.String(),
					},
					Spec: v1.CompositionSpec{
						Pipeline: []v1.PipelineStep{{
							Step: "auto-ready",
							FunctionRef: v1.FunctionReference{
								Name: "crossplane-contrib-function-auto-ready",
							},
							Input: &runtime.RawExtension{
								Raw: []byte(`{"apiVersion": "v1", "kind": "NotInput"}`),
							},
						}},
					},
				},
			},
			want: want{
				result: &validate.Result{
					Errors: []error{
						&validator.ValidationError{
							TypeCode: 100,
							Message:  `incorrect input type for step "auto-ready"; valid inputs: [my-function.com/v1alpha1, Kind=Input]`,
							Name:     "spec.pipeline[0].input.apiVersion",
						},
					},
				},
			},
		},
		"FunctionRefInvalidInput": {
			reason: "Invalid input to a function step should produce a validation error.",
			args: args{
				workspace: func() *workspace.Workspace {
					f := afero.NewMemMapFs()
					_ = afero.WriteFile(f, "/upbound.yaml", projectFile, 0o644)
					ws, _ := workspace.New("/", workspace.WithFS(f))
					return ws
				}(),
				packages: map[string]*mxpkg.ParsedPackage{
					"xpkg.upbound.io/crossplane-contrib/function-auto-ready": {
						PType: v1beta1.FunctionPackageType,
						Objs: []runtime.Object{
							&apiextv1.CustomResourceDefinition{
								TypeMeta: apimetav1.TypeMeta{
									APIVersion: apiextv1.SchemeGroupVersion.String(),
									Kind:       "CustomResourceDefinition",
								},
								ObjectMeta: apimetav1.ObjectMeta{
									Name: "input.my-function.com",
								},
								Spec: apiextv1.CustomResourceDefinitionSpec{
									Group: "my-function.com",
									Names: apiextv1.CustomResourceDefinitionNames{
										Plural:   "inputs",
										Singular: "input",
										Kind:     "Input",
										ListKind: "InputList",
									},
									Versions: []apiextv1.CustomResourceDefinitionVersion{{
										Name:    "v1alpha1",
										Served:  true,
										Storage: true,
										Schema: &apiextv1.CustomResourceValidation{
											OpenAPIV3Schema: &apiextv1.JSONSchemaProps{
												Properties: map[string]apiextv1.JSONSchemaProps{
													"apiVersion": {
														Type: "string",
													},
													"kind": {
														Type: "string",
													},
													"boolField": {
														Type: "boolean",
													},
												},
											},
										},
									}},
								},
							},
						},
					},
				},
				data: &v1.Composition{
					TypeMeta: apimetav1.TypeMeta{
						Kind:       v1.CompositionKind,
						APIVersion: v1.SchemeGroupVersion.String(),
					},
					Spec: v1.CompositionSpec{
						Pipeline: []v1.PipelineStep{{
							Step: "auto-ready",
							FunctionRef: v1.FunctionReference{
								Name: "crossplane-contrib-function-auto-ready",
							},
							Input: &runtime.RawExtension{
								Raw: []byte(`{"apiVersion": "my-function.com/v1alpha1", "kind": "Input", "boolField": "asdf"}`),
							},
						}},
					},
				},
			},
			want: want{
				result: &validate.Result{
					Errors: []error{
						&validator.ValidationError{
							TypeCode: 601,
							Message:  `boolField in body must be of type boolean: "string" (my-function.com/v1alpha1, Kind=Input)`,
							Name:     "spec.pipeline[0].input.boolField",
						},
					},
				},
			},
		},
		"FunctionRefValidInput": {
			reason: "A pipeline step with valid input should not produce errors.",
			args: args{
				workspace: func() *workspace.Workspace {
					f := afero.NewMemMapFs()
					_ = afero.WriteFile(f, "/upbound.yaml", projectFile, 0o644)
					ws, _ := workspace.New("/", workspace.WithFS(f))
					return ws
				}(),
				packages: map[string]*mxpkg.ParsedPackage{
					"xpkg.upbound.io/crossplane-contrib/function-auto-ready": {
						PType: v1beta1.FunctionPackageType,
						Objs: []runtime.Object{
							&apiextv1.CustomResourceDefinition{
								TypeMeta: apimetav1.TypeMeta{
									APIVersion: apiextv1.SchemeGroupVersion.String(),
									Kind:       "CustomResourceDefinition",
								},
								ObjectMeta: apimetav1.ObjectMeta{
									Name: "input.my-function.com",
								},
								Spec: apiextv1.CustomResourceDefinitionSpec{
									Group: "my-function.com",
									Names: apiextv1.CustomResourceDefinitionNames{
										Plural:   "inputs",
										Singular: "input",
										Kind:     "Input",
										ListKind: "InputList",
									},
									Versions: []apiextv1.CustomResourceDefinitionVersion{{
										Name:    "v1alpha1",
										Served:  true,
										Storage: true,
										Schema: &apiextv1.CustomResourceValidation{
											OpenAPIV3Schema: &apiextv1.JSONSchemaProps{
												Properties: map[string]apiextv1.JSONSchemaProps{
													"apiVersion": {
														Type: "string",
													},
													"kind": {
														Type: "string",
													},
													"boolField": {
														Type: "boolean",
													},
												},
											},
										},
									}},
								},
							},
						},
					},
				},
				data: &v1.Composition{
					TypeMeta: apimetav1.TypeMeta{
						Kind:       v1.CompositionKind,
						APIVersion: v1.SchemeGroupVersion.String(),
					},
					Spec: v1.CompositionSpec{
						Pipeline: []v1.PipelineStep{{
							Step: "auto-ready",
							FunctionRef: v1.FunctionReference{
								Name: "crossplane-contrib-function-auto-ready",
							},
							Input: &runtime.RawExtension{
								Raw: []byte(`{"apiVersion": "my-function.com/v1alpha1", "kind": "Input", "boolField": true}`),
							},
						}},
					},
				},
			},
			want: want{
				result: &validate.Result{},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s.w = tc.args.workspace
			if err := s.w.Parse(ctx); err != nil {
				t.Fatalf("failed to parse workspace for test: %v", err)
			}
			s.wsview = s.w.View()
			s.validators = tc.args.validators
			s.packages = tc.args.packages

			// convert runtime.Object -> *unstructured.Unstructured
			b, err := json.Marshal(tc.args.data)
			// we shouldn't see an error from Marshaling
			if diff := cmp.Diff(err, nil, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nCompositionValidation(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			var u unstructured.Unstructured
			json.Unmarshal(b, &u)

			v, _ := DefaultCompositionValidators(s)

			result := v.Validate(ctx, &u)

			if diff := cmp.Diff(tc.want.result, result); diff != "" {
				t.Errorf("\n%s\nCompositionValidation(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
