// Copyright 2025 Upbound Inc.
// All rights reserved

package ctp

import (
	"testing"

	openapi_v2 "github.com/google/gnostic-models/openapiv2"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/openapi"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestFindTestResources(t *testing.T) {
	t.Parallel()

	// Create a mock discovery client that returns test API resources
	mockDiscovery := &mockDiscoveryInterface{
		resources: []*metav1.APIResourceList{
			{
				GroupVersion: "example.io/v1",
				APIResources: []metav1.APIResource{
					{
						Name:       "testclaims",
						Kind:       "TestClaim",
						Categories: []string{"claim"},
						Verbs:      []string{"list", "delete"},
					},
				},
			},
			{
				GroupVersion: "example.io/v1alpha1",
				APIResources: []metav1.APIResource{
					{
						Name:       "testcomposites",
						Kind:       "TestComposite",
						Categories: []string{"composite"},
						Verbs:      []string{"list", "delete"},
					},
				},
			},
			{
				GroupVersion: "provider.io/v1",
				APIResources: []metav1.APIResource{
					{
						Name:       "buckets",
						Kind:       "Bucket",
						Categories: []string{"managed"},
						Verbs:      []string{"list", "delete"},
					},
				},
			},
		},
	}

	tests := map[string]struct {
		name          string
		mockResources []runtime.Object
		want          map[string]GenericResource
		wantErr       bool
	}{
		"FindsClaimWithTestAnnotation": {
			mockResources: []runtime.Object{
				&unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "example.io/v1",
						"kind":       "TestClaim",
						"metadata": map[string]any{
							"name": "test-claim-1",
							"annotations": map[string]any{
								"cli.upbound.io/e2etest": "True",
							},
						},
						"spec": map[string]any{},
					},
				},
			},
			want: map[string]GenericResource{
				"example.io/v1/TestClaim/test-claim-1": {
					GVK: metav1.GroupVersionKind{
						Group:   "example.io",
						Version: "v1",
						Kind:    "TestClaim",
					},
					Name:    "test-claim-1",
					Status:  resourceStatusPending,
					Message: "test resource",
				},
			},
		},
		"FindsCompositeWithManagedResources": {
			mockResources: []runtime.Object{
				&unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "example.io/v1alpha1",
						"kind":       "TestComposite",
						"metadata": map[string]any{
							"name":      "test-composite-1",
							"namespace": "default",
							"annotations": map[string]any{
								"cli.upbound.io/e2etest": "True",
							},
						},
						"spec": map[string]any{
							"crossplane": map[string]any{
								"resourceRefs": []any{
									map[string]any{
										"apiVersion": "provider.io/v1",
										"kind":       "Bucket",
										"name":       "managed-bucket-1",
									},
								},
							},
						},
					},
				},
			},
			want: map[string]GenericResource{
				"example.io/v1alpha1/TestComposite/test-composite-1": {
					GVK: metav1.GroupVersionKind{
						Group:   "example.io",
						Version: "v1alpha1",
						Kind:    "TestComposite",
					},
					Name:      "test-composite-1",
					Namespace: "default",
					Status:    resourceStatusPending,
					Message:   "test resource",
				},
				"provider.io/v1/Bucket/managed-bucket-1": {
					GVK: metav1.GroupVersionKind{
						Group:   "provider.io",
						Version: "v1",
						Kind:    "Bucket",
					},
					Name:    "managed-bucket-1",
					Status:  resourceStatusPending,
					Message: "managed resource",
				},
			},
		},
		"IgnoresResourcesWithoutTestAnnotation": {
			mockResources: []runtime.Object{
				&unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "example.io/v1",
						"kind":       "TestClaim",
						"metadata": map[string]any{
							"name": "regular-claim",
						},
						"spec": map[string]any{},
					},
				},
				&unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "example.io/v1",
						"kind":       "TestClaim",
						"metadata": map[string]any{
							"name": "test-claim-2",
							"annotations": map[string]any{
								"cli.upbound.io/e2etest": "True",
							},
						},
						"spec": map[string]any{},
					},
				},
			},
			want: map[string]GenericResource{
				"example.io/v1/TestClaim/test-claim-2": {
					GVK: metav1.GroupVersionKind{
						Group:   "example.io",
						Version: "v1",
						Kind:    "TestClaim",
					},
					Name:    "test-claim-2",
					Status:  resourceStatusPending,
					Message: "test resource",
				},
			},
		},
		"DeduplicatesManagedResources": {
			mockResources: []runtime.Object{
				&unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "example.io/v1alpha1",
						"kind":       "TestComposite",
						"metadata": map[string]any{
							"name": "composite-1",
							"annotations": map[string]any{
								"cli.upbound.io/e2etest": "True",
							},
						},
						"spec": map[string]any{
							"resourceRefs": []any{
								map[string]any{
									"apiVersion": "provider.io/v1",
									"kind":       "Bucket",
									"name":       "shared-bucket",
								},
							},
						},
					},
				},
				&unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "example.io/v1alpha1",
						"kind":       "TestComposite",
						"metadata": map[string]any{
							"name": "composite-2",
							"annotations": map[string]any{
								"cli.upbound.io/e2etest": "True",
							},
						},
						"spec": map[string]any{
							"resourceRefs": []any{
								map[string]any{
									"apiVersion": "provider.io/v1",
									"kind":       "Bucket",
									"name":       "shared-bucket",
								},
							},
						},
					},
				},
			},
			want: map[string]GenericResource{
				"example.io/v1alpha1/TestComposite/composite-1": {
					GVK: metav1.GroupVersionKind{
						Group:   "example.io",
						Version: "v1alpha1",
						Kind:    "TestComposite",
					},
					Name:    "composite-1",
					Status:  resourceStatusPending,
					Message: "test resource",
				},
				"example.io/v1alpha1/TestComposite/composite-2": {
					GVK: metav1.GroupVersionKind{
						Group:   "example.io",
						Version: "v1alpha1",
						Kind:    "TestComposite",
					},
					Name:    "composite-2",
					Status:  resourceStatusPending,
					Message: "test resource",
				},
				"provider.io/v1/Bucket/shared-bucket": {
					GVK: metav1.GroupVersionKind{
						Group:   "provider.io",
						Version: "v1",
						Kind:    "Bucket",
					},
					Name:    "shared-bucket",
					Status:  resourceStatusPending,
					Message: "managed resource",
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Create fake client with mock resources
			fakeClient := fake.NewClientBuilder().
				WithRuntimeObjects(tc.mockResources...).
				Build()

			helper := &cleanupHelper{
				client: fakeClient,
			}

			got, err := helper.findTestResources(t.Context(), mockDiscovery, "cli.upbound.io/e2etest")

			if tc.wantErr {
				assert.Assert(t, err != nil)
			} else {
				assert.NilError(t, err)
				assert.DeepEqual(t, got, tc.want)
			}
		})
	}
}

// mockDiscoveryInterface is a mock implementation of discovery.DiscoveryInterface.
type mockDiscoveryInterface struct {
	resources []*metav1.APIResourceList
}

func (m *mockDiscoveryInterface) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	return m.resources, nil
}

// Implement other required methods with no-op.
func (m *mockDiscoveryInterface) RESTClient() rest.Interface                  { return nil }
func (m *mockDiscoveryInterface) ServerGroups() (*metav1.APIGroupList, error) { return nil, nil }
func (m *mockDiscoveryInterface) ServerResourcesForGroupVersion(_ string) (*metav1.APIResourceList, error) {
	return nil, nil
}

func (m *mockDiscoveryInterface) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	return nil, nil, nil
}

func (m *mockDiscoveryInterface) ServerPreferredNamespacedResources() ([]*metav1.APIResourceList, error) {
	return nil, nil
}
func (m *mockDiscoveryInterface) ServerVersion() (*version.Info, error)        { return nil, nil }
func (m *mockDiscoveryInterface) OpenAPISchema() (*openapi_v2.Document, error) { return nil, nil }
func (m *mockDiscoveryInterface) OpenAPIV3() openapi.Client                    { return nil }
func (m *mockDiscoveryInterface) WithLegacy() discovery.DiscoveryInterface     { return m }

func TestGetResourceKey(t *testing.T) {
	t.Parallel()
	helper := &cleanupHelper{}

	tests := map[string]struct {
		resource GenericResource
		want     string
	}{
		"WithNamespace": {
			resource: GenericResource{
				GVK: metav1.GroupVersionKind{
					Group:   "example.io",
					Version: "v1",
					Kind:    "TestResource",
				},
				Name:      "test-1",
				Namespace: "default",
			},
			want: "example.io/v1/TestResource/test-1",
		},
		"WithoutNamespace": {
			resource: GenericResource{
				GVK: metav1.GroupVersionKind{
					Group:   "example.io",
					Version: "v1",
					Kind:    "TestResource",
				},
				Name: "test-1",
			},
			want: "example.io/v1/TestResource/test-1",
		},
		"EmptyGroup": {
			resource: GenericResource{
				GVK: metav1.GroupVersionKind{
					Group:   "",
					Version: "v1",
					Kind:    "Pod",
				},
				Name: "my-pod",
			},
			want: "/v1/Pod/my-pod",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := helper.getResourceKey(tc.resource)
			assert.Equal(t, got, tc.want)
		})
	}
}

func TestCreateUnstructuredObject(t *testing.T) { //nolint:tparallel // See comment below.
	// Note: Cannot use t.Parallel() here due to concurrent map write issues
	// in the fake client scheme registry when tests run in parallel
	helper := &cleanupHelper{}

	tests := map[string]struct {
		resource GenericResource
		validate func(t *testing.T, obj *unstructured.Unstructured)
	}{
		"WithNamespace": {
			resource: GenericResource{
				GVK: metav1.GroupVersionKind{
					Group:   "example.io",
					Version: "v1",
					Kind:    "TestResource",
				},
				Name:      "test-1",
				Namespace: "default",
			},
			validate: func(t *testing.T, obj *unstructured.Unstructured) {
				assert.Equal(t, obj.GetName(), "test-1")
				assert.Equal(t, obj.GetNamespace(), "default")
				assert.Equal(t, obj.GetKind(), "TestResource")
				gvk := obj.GroupVersionKind()
				assert.Equal(t, gvk.Group, "example.io")
				assert.Equal(t, gvk.Version, "v1")
			},
		},
		"WithoutNamespace": {
			resource: GenericResource{
				GVK: metav1.GroupVersionKind{
					Group:   "example.io",
					Version: "v1",
					Kind:    "ClusterResource",
				},
				Name: "cluster-1",
			},
			validate: func(t *testing.T, obj *unstructured.Unstructured) {
				assert.Equal(t, obj.GetName(), "cluster-1")
				assert.Equal(t, obj.GetNamespace(), "")
				assert.Equal(t, obj.GetKind(), "ClusterResource")
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := helper.createUnstructuredObject(tc.resource)
			tc.validate(t, got)
		})
	}
}

func TestGetSyncedConditionMessage(t *testing.T) {
	t.Parallel()

	helper := &cleanupHelper{}

	tests := map[string]struct {
		item *unstructured.Unstructured
		want string
	}{
		"WithSyncedCondition": {
			item: &unstructured.Unstructured{
				Object: map[string]any{
					"status": map[string]any{
						"conditions": []any{
							map[string]any{
								"type":    "Ready",
								"message": "Resource is ready",
							},
							map[string]any{
								"type":    "Synced",
								"message": "Cannot connect to provider",
							},
						},
					},
				},
			},
			want: "Cannot connect to provider",
		},
		"NoSyncedCondition": {
			item: &unstructured.Unstructured{
				Object: map[string]any{
					"status": map[string]any{
						"conditions": []any{
							map[string]any{
								"type":    "Ready",
								"message": "Resource is ready",
							},
						},
					},
				},
			},
			want: "",
		},
		"NoConditions": {
			item: &unstructured.Unstructured{
				Object: map[string]any{
					"status": map[string]any{},
				},
			},
			want: "",
		},
		"NoStatus": {
			item: &unstructured.Unstructured{
				Object: map[string]any{},
			},
			want: "",
		},
		"InvalidStatusType": {
			item: &unstructured.Unstructured{
				Object: map[string]any{
					"status": "invalid",
				},
			},
			want: "",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := helper.getSyncedConditionMessage(tc.item)
			assert.Equal(t, got, tc.want)
		})
	}
}

func TestGetExternalName(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		item *unstructured.Unstructured
		want string
	}{
		"WithExternalName": {
			item: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"annotations": map[string]any{
							"crossplane.io/external-name": "aws-bucket-12345",
							"other-annotation":            "value",
						},
					},
				},
			},
			want: "aws-bucket-12345",
		},
		"NoExternalName": {
			item: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"annotations": map[string]any{
							"other-annotation": "value",
						},
					},
				},
			},
			want: "",
		},
		"NoAnnotations": {
			item: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{},
				},
			},
			want: "",
		},
		"NoMetadata": {
			item: &unstructured.Unstructured{
				Object: map[string]any{},
			},
			want: "",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := getExternalName(tc.item)
			assert.Equal(t, got, tc.want)
		})
	}
}
