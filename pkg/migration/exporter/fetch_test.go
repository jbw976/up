// Copyright 2025 Upbound Inc.
// All rights reserved

package exporter

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestUnstructuredFetcherShouldSkip(t *testing.T) {
	type args struct {
		includedNamespaces map[string]struct{}
		excludedNamespaces map[string]struct{}
		r                  unstructured.Unstructured
	}
	type want struct {
		skip bool
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"SkipNonIncludedNamespaces": {
			args: args{
				includedNamespaces: map[string]struct{}{
					"foo": {},
				},
				r: unstructured.Unstructured{
					Object: map[string]interface{}{
						"kind": "Namespace",
						"metadata": map[string]interface{}{
							"name": "bar",
						},
					},
				},
			},
			want: want{
				skip: true,
			},
		},
		"SkipExcludedNamespaces": {
			args: args{
				excludedNamespaces: map[string]struct{}{
					"bar": {},
				},
				r: unstructured.Unstructured{
					Object: map[string]interface{}{
						"kind": "Namespace",
						"metadata": map[string]interface{}{
							"name": "bar",
						},
					},
				},
			},
			want: want{
				skip: true,
			},
		},
		"DontSkipIncludedNamespaces": {
			args: args{
				includedNamespaces: map[string]struct{}{
					"bar": {},
				},
				r: unstructured.Unstructured{
					Object: map[string]interface{}{
						"kind": "Namespace",
						"metadata": map[string]interface{}{
							"name": "bar",
						},
					},
				},
			},
			want: want{
				skip: false,
			},
		},
		"DontSkipIfNotAnExcludedNamespace": {
			args: args{
				excludedNamespaces: map[string]struct{}{
					"foo": {},
				},
				r: unstructured.Unstructured{
					Object: map[string]interface{}{
						"kind": "Namespace",
						"metadata": map[string]interface{}{
							"name": "bar",
						},
					},
				},
			},
			want: want{
				skip: false,
			},
		},

		"SkipNonIncludedNamespacedResource": {
			args: args{
				includedNamespaces: map[string]struct{}{
					"foo": {},
				},
				r: unstructured.Unstructured{
					Object: map[string]interface{}{
						"kind": "Some",
						"metadata": map[string]interface{}{
							"namespace": "bar",
						},
					},
				},
			},
			want: want{
				skip: true,
			},
		},
		"SkipExcludedNamespacedResource": {
			args: args{
				excludedNamespaces: map[string]struct{}{
					"bar": {},
				},
				r: unstructured.Unstructured{
					Object: map[string]interface{}{
						"kind": "Some",
						"metadata": map[string]interface{}{
							"namespace": "bar",
						},
					},
				},
			},
			want: want{
				skip: true,
			},
		},
		"DontSkipIncludedNamespacedResource": {
			args: args{
				includedNamespaces: map[string]struct{}{
					"bar": {},
				},
				r: unstructured.Unstructured{
					Object: map[string]interface{}{
						"kind": "Some",
						"metadata": map[string]interface{}{
							"namespace": "bar",
						},
					},
				},
			},
			want: want{
				skip: false,
			},
		},
		"DontSkipIfNotAnExcludedNamespacedResource": {
			args: args{
				excludedNamespaces: map[string]struct{}{
					"foo": {},
				},
				r: unstructured.Unstructured{
					Object: map[string]interface{}{
						"kind": "Some",
						"metadata": map[string]interface{}{
							"namespace": "bar",
						},
					},
				},
			},
			want: want{
				skip: false,
			},
		},

		"SkipKubeRootCA": {
			args: args{
				r: unstructured.Unstructured{
					Object: map[string]interface{}{
						"kind": "ConfigMap",
						"metadata": map[string]interface{}{
							"name": "kube-root-ca.crt",
						},
					},
				},
			},
			want: want{
				skip: true,
			},
		},

		"SkipHelmSecret": {
			args: args{
				r: unstructured.Unstructured{
					Object: map[string]interface{}{
						"kind": "Secret",
						"type": "helm.sh/release.v1",
					},
				},
			},
			want: want{
				skip: true,
			},
		},

		"SkipPackageManagerOwnedResources": {
			args: args{
				r: unstructured.Unstructured{
					Object: map[string]interface{}{
						"kind": "Some",
						"metadata": map[string]interface{}{
							"ownerReferences": []interface{}{
								map[string]interface{}{
									"apiVersion": "pkg.crossplane.io/v1",
								},
							},
						},
					},
				},
			},
			want: want{
				skip: true,
			},
		},

		"SkipPackageManagerLock": {
			args: args{
				r: unstructured.Unstructured{
					Object: map[string]interface{}{
						"kind":       "Lock",
						"apiVersion": "pkg.crossplane.io/v1beta1",
					},
				},
			},
			want: want{
				skip: true,
			},
		},

		"DontSkipAnythingElse": {
			args: args{
				r: unstructured.Unstructured{
					Object: map[string]interface{}{
						"kind": "Some",
					},
				},
			},
			want: want{
				skip: false,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := &UnstructuredFetcher{
				includedNamespaces: tc.args.includedNamespaces,
				excludedNamespaces: tc.args.excludedNamespaces,
			}
			if diff := cmp.Diff(e.shouldSkip(tc.args.r), tc.want.skip); diff != "" {
				t.Errorf("shouldSkip() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
