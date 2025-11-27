// Copyright 2025 Upbound Inc.
// All rights reserved

package counter

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestClassifyCRD(t *testing.T) {
	tests := map[string]struct {
		crd  apiextensionsv1.CustomResourceDefinition
		want resourceType
	}{
		"ManagedResource": {
			crd: apiextensionsv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "pkg.crossplane.io/v1",
							Kind:       "ProviderRevision",
						},
					},
				},
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Names: apiextensionsv1.CustomResourceDefinitionNames{
						Kind: "Bucket",
					},
				},
			},
			want: resourceTypeManagedResource,
		},
		"ProviderConfigExcluded": {
			crd: apiextensionsv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "pkg.crossplane.io/v1",
							Kind:       "ProviderRevision",
						},
					},
				},
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Names: apiextensionsv1.CustomResourceDefinitionNames{
						Kind: "ProviderConfig",
					},
				},
			},
			want: resourceTypeExcluded,
		},
		"CompositeResource": {
			crd: apiextensionsv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "apiextensions.crossplane.io/v1",
							Kind:       "CompositeResourceDefinition",
						},
					},
				},
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Names: apiextensionsv1.CustomResourceDefinitionNames{
						Kind:       "XDatabase",
						Categories: []string{"composite"},
					},
				},
			},
			want: resourceTypeComposite,
		},
		"CompositeResourceClaim": {
			crd: apiextensionsv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "apiextensions.crossplane.io/v1",
							Kind:       "CompositeResourceDefinition",
						},
					},
				},
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Names: apiextensionsv1.CustomResourceDefinitionNames{
						Kind:       "Database",
						Categories: []string{"claim"},
					},
				},
			},
			want: resourceTypeClaim,
		},
		"ExcludedNoOwnerRef": {
			crd: apiextensionsv1.CustomResourceDefinition{
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Names: apiextensionsv1.CustomResourceDefinitionNames{
						Kind: "SomeOtherResource",
					},
				},
			},
			want: resourceTypeExcluded,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := classifyCRD(tc.crd)
			if got != tc.want {
				t.Errorf("classifyCRD() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCreateResourceKey(t *testing.T) {
	tests := map[string]struct {
		res  unstructured.Unstructured
		want string
	}{
		"ClusterScoped": {
			res: unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "s3.aws.upbound.io/v1beta1",
					"kind":       "Bucket",
					"metadata": map[string]any{
						"name": "my-bucket",
					},
				},
			},
			want: "s3.aws.upbound.io/v1beta1/Bucket:my-bucket",
		},
		"NamespaceScoped": {
			res: unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]any{
						"name":      "my-configmap",
						"namespace": "default",
					},
				},
			},
			want: "v1/ConfigMap:default/my-configmap",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := createResourceKey(tc.res)
			if got != tc.want {
				t.Errorf("createResourceKey() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestExtractComposedResourceKeys(t *testing.T) {
	tests := map[string]struct {
		res  unstructured.Unstructured
		want []string
	}{
		"WithResourceRefs": {
			res: unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "example.org/v1",
					"kind":       "XDatabase",
					"metadata": map[string]any{
						"name": "my-db",
					},
					"spec": map[string]any{
						"resourceRefs": []any{
							map[string]any{
								"apiVersion": "rds.aws.upbound.io/v1beta1",
								"kind":       "Instance",
								"name":       "my-db-instance",
							},
							map[string]any{
								"apiVersion": "rds.aws.upbound.io/v1beta1",
								"kind":       "SubnetGroup",
								"name":       "my-db-subnet",
							},
						},
					},
				},
			},
			want: []string{
				"rds.aws.upbound.io/v1beta1/Instance:my-db-instance",
				"rds.aws.upbound.io/v1beta1/SubnetGroup:my-db-subnet",
			},
		},
		"NoResourceRefs": {
			res: unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "example.org/v1",
					"kind":       "XDatabase",
					"metadata": map[string]any{
						"name": "my-db",
					},
					"spec": map[string]any{},
				},
			},
			want: nil,
		},
		"EmptyResourceRefs": {
			res: unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "example.org/v1",
					"kind":       "XDatabase",
					"metadata": map[string]any{
						"name": "my-db",
					},
					"spec": map[string]any{
						"resourceRefs": []any{},
					},
				},
			},
			want: nil,
		},
		"CrossplaneV2Path": {
			res: unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "platform.upbound.io/v1",
					"kind":       "App",
					"metadata": map[string]any{
						"name":      "example",
						"namespace": "default",
					},
					"spec": map[string]any{
						"crossplane": map[string]any{
							"resourceRefs": []any{
								map[string]any{
									"apiVersion": "apps/v1",
									"kind":       "Deployment",
									"name":       "app1",
								},
								map[string]any{
									"apiVersion": "v1",
									"kind":       "Service",
									"name":       "app1",
								},
							},
						},
					},
				},
			},
			want: []string{
				"apps/v1/Deployment:app1",
				"v1/Service:app1",
			},
		},
		"NamespacedComposedResources": {
			res: unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "platform.upbound.io/v1",
					"kind":       "App",
					"metadata": map[string]any{
						"name":      "example",
						"namespace": "default",
					},
					"spec": map[string]any{
						"crossplane": map[string]any{
							"resourceRefs": []any{
								map[string]any{
									"apiVersion": "apps/v1",
									"kind":       "Deployment",
									"name":       "app1",
									"namespace":  "prod",
								},
								map[string]any{
									"apiVersion": "apps/v1",
									"kind":       "Deployment",
									"name":       "app1",
									"namespace":  "staging",
								},
							},
						},
					},
				},
			},
			want: []string{
				"apps/v1/Deployment:prod/app1",
				"apps/v1/Deployment:staging/app1",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := extractComposedResourceKeys(tc.res)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("extractComposedResourceKeys() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGetStorageGVR(t *testing.T) {
	tests := map[string]struct {
		crd     apiextensionsv1.CustomResourceDefinition
		wantGVR schema.GroupVersionResource
		wantOK  bool
	}{
		"WithStorageVersion": {
			crd: apiextensionsv1.CustomResourceDefinition{
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Group: "s3.aws.upbound.io",
					Names: apiextensionsv1.CustomResourceDefinitionNames{
						Plural: "buckets",
					},
					Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
						{Name: "v1alpha1", Storage: false},
						{Name: "v1beta1", Storage: true},
						{Name: "v1", Storage: false},
					},
				},
			},
			wantGVR: schema.GroupVersionResource{
				Group:    "s3.aws.upbound.io",
				Version:  "v1beta1",
				Resource: "buckets",
			},
			wantOK: true,
		},
		"NoStorageVersion": {
			crd: apiextensionsv1.CustomResourceDefinition{
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Group: "example.io",
					Names: apiextensionsv1.CustomResourceDefinitionNames{
						Plural: "examples",
					},
					Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
						{Name: "v1", Storage: false},
					},
				},
			},
			wantGVR: schema.GroupVersionResource{},
			wantOK:  false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			gvr, ok := getStorageGVR(tc.crd)
			if ok != tc.wantOK {
				t.Errorf("ok = %v, want %v", ok, tc.wantOK)
			}
			if diff := cmp.Diff(tc.wantGVR, gvr); diff != "" {
				t.Errorf("getStorageGVR() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestIsProviderConfig(t *testing.T) {
	tests := map[string]struct {
		kind string
		want bool
	}{
		"ProviderConfig":        {kind: "ProviderConfig", want: true},
		"ClusterProviderConfig": {kind: "ClusterProviderConfig", want: true},
		"ProviderConfigUsage":   {kind: "ProviderConfigUsage", want: true},
		"Bucket":                {kind: "Bucket", want: false},
		"Instance":              {kind: "Instance", want: false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := isProviderConfig(tc.kind)
			if got != tc.want {
				t.Errorf("isProviderConfig(%q) = %v, want %v", tc.kind, got, tc.want)
			}
		})
	}
}
