// Copyright 2025 Upbound Inc.
// All rights reserved

package importer

import (
	"context"
	"errors"
	"testing"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
)

func TestApplyResources(t *testing.T) {
	type args struct {
		resources   []unstructured.Unstructured
		applyStatus bool
	}

	type fields struct {
		dynamicClient  dynamic.Interface
		resourceMapper meta.RESTMapper
	}

	type want struct {
		err error
	}

	// Setup test resource
	testResource := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "test.upbound.io/v1",
			"kind":       "TestResource",
			"metadata": map[string]interface{}{
				"name":      "test-resource",
				"namespace": "test-namespace",
			},
			"spec": map[string]interface{}{
				"foo": "bar",
			},
		},
	}

	// Setup test error
	testError := errors.New("test error")
	// Reset counters for this test
	applyAttemptCount := 0
	statusAttemptCount := 0

	cases := map[string]struct {
		args   args
		fields fields
		want   want
	}{
		"SuccessWithoutStatus": {
			args: args{
				resources:   []unstructured.Unstructured{testResource},
				applyStatus: false,
			},
			fields: fields{
				resourceMapper: &mockRESTMapper{
					mappingFunc: func(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
						return &meta.RESTMapping{
							Resource: schema.GroupVersionResource{
								Group:    "test.upbound.io",
								Version:  "v1",
								Resource: "testresources",
							},
						}, nil
					},
				},
				dynamicClient: &mockDynamicInterface{
					resourceFunc: func(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
						return &mockNamespaceableResourceInterface{
							namespaceFunc: func(ns string) dynamic.ResourceInterface {
								return &mockResourceInterface{
									applyFunc: func(ctx context.Context, name string, obj *unstructured.Unstructured, options v1.ApplyOptions, subresources ...string) (*unstructured.Unstructured, error) {
										return obj, nil
									},
								}
							},
						}
					},
				},
			},
			want: want{
				err: nil,
			},
		},
		"SuccessWithStatus": {
			args: args{
				resources:   []unstructured.Unstructured{testResource},
				applyStatus: true,
			},
			fields: fields{
				resourceMapper: &mockRESTMapper{
					mappingFunc: func(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
						return &meta.RESTMapping{
							Resource: schema.GroupVersionResource{
								Group:    "test.upbound.io",
								Version:  "v1",
								Resource: "testresources",
							},
						}, nil
					},
				},
				dynamicClient: &mockDynamicInterface{
					resourceFunc: func(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
						return &mockNamespaceableResourceInterface{
							namespaceFunc: func(ns string) dynamic.ResourceInterface {
								return &mockResourceInterface{
									applyFunc: func(ctx context.Context, name string, obj *unstructured.Unstructured, options v1.ApplyOptions, subresources ...string) (*unstructured.Unstructured, error) {
										return obj, nil
									},
									applyStatusFunc: func(ctx context.Context, name string, obj *unstructured.Unstructured, options v1.ApplyOptions) (*unstructured.Unstructured, error) {
										return obj, nil
									},
								}
							},
						}
					},
				},
			},
			want: want{
				err: nil,
			},
		},
		"ErrorOnRESTMapping": {
			args: args{
				resources:   []unstructured.Unstructured{testResource},
				applyStatus: false,
			},
			fields: fields{
				resourceMapper: &mockRESTMapper{
					mappingFunc: func(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
						return nil, testError
					},
				},
				dynamicClient: &mockDynamicInterface{},
			},
			want: want{
				err: testError,
			},
		},
		"ErrorOnApply": {
			args: args{
				resources:   []unstructured.Unstructured{testResource},
				applyStatus: false,
			},
			fields: fields{
				resourceMapper: &mockRESTMapper{
					mappingFunc: func(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
						return &meta.RESTMapping{
							Resource: schema.GroupVersionResource{
								Group:    "test.upbound.io",
								Version:  "v1",
								Resource: "testresources",
							},
						}, nil
					},
				},
				dynamicClient: &mockDynamicInterface{
					resourceFunc: func(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
						return &mockNamespaceableResourceInterface{
							namespaceFunc: func(ns string) dynamic.ResourceInterface {
								return &mockResourceInterface{
									applyFunc: func(ctx context.Context, name string, obj *unstructured.Unstructured, options v1.ApplyOptions, subresources ...string) (*unstructured.Unstructured, error) {
										return nil, testError
									},
								}
							},
						}
					},
				},
			},
			want: want{
				err: testError,
			},
		},
		"ErrorOnApplyStatus": {
			args: args{
				resources:   []unstructured.Unstructured{testResource},
				applyStatus: true,
			},
			fields: fields{
				resourceMapper: &mockRESTMapper{
					mappingFunc: func(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
						return &meta.RESTMapping{
							Resource: schema.GroupVersionResource{
								Group:    "test.upbound.io",
								Version:  "v1",
								Resource: "testresources",
							},
						}, nil
					},
				},
				dynamicClient: &mockDynamicInterface{
					resourceFunc: func(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
						return &mockNamespaceableResourceInterface{
							namespaceFunc: func(ns string) dynamic.ResourceInterface {
								return &mockResourceInterface{
									applyFunc: func(ctx context.Context, name string, obj *unstructured.Unstructured, options v1.ApplyOptions, subresources ...string) (*unstructured.Unstructured, error) {
										return obj, nil
									},
									applyStatusFunc: func(ctx context.Context, name string, obj *unstructured.Unstructured, options v1.ApplyOptions) (*unstructured.Unstructured, error) {
										return nil, testError
									},
								}
							},
						}
					},
				},
			},
			want: want{
				err: testError,
			},
		},
		"IgnoreNotFoundOnApplyStatus": {
			args: args{
				resources:   []unstructured.Unstructured{testResource},
				applyStatus: true,
			},
			fields: fields{
				resourceMapper: &mockRESTMapper{
					mappingFunc: func(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
						return &meta.RESTMapping{
							Resource: schema.GroupVersionResource{
								Group:    "test.upbound.io",
								Version:  "v1",
								Resource: "testresources",
							},
						}, nil
					},
				},
				dynamicClient: &mockDynamicInterface{
					resourceFunc: func(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
						return &mockNamespaceableResourceInterface{
							namespaceFunc: func(ns string) dynamic.ResourceInterface {
								return &mockResourceInterface{
									applyFunc: func(ctx context.Context, name string, obj *unstructured.Unstructured, options v1.ApplyOptions, subresources ...string) (*unstructured.Unstructured, error) {
										return obj, nil
									},
									applyStatusFunc: func(ctx context.Context, name string, obj *unstructured.Unstructured, options v1.ApplyOptions) (*unstructured.Unstructured, error) {
										return nil, k8serrors.NewNotFound(schema.GroupResource{Group: "test.upbound.io", Resource: "testresources"}, "test-resource")
									},
								}
							},
						}
					},
				},
			},
			want: want{
				err: nil,
			},
		},
		"RetryOnApply": {
			args: args{
				resources:   []unstructured.Unstructured{testResource},
				applyStatus: false,
			},
			fields: fields{
				resourceMapper: &mockRESTMapper{
					mappingFunc: func(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
						return &meta.RESTMapping{
							Resource: schema.GroupVersionResource{
								Group:    "test.upbound.io",
								Version:  "v1",
								Resource: "testresources",
							},
						}, nil
					},
				},
				dynamicClient: &mockDynamicInterface{
					resourceFunc: func(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
						return &mockNamespaceableResourceInterface{
							namespaceFunc: func(ns string) dynamic.ResourceInterface {
								return &mockResourceInterface{
									applyFunc: func(ctx context.Context, name string, obj *unstructured.Unstructured, options v1.ApplyOptions, subresources ...string) (*unstructured.Unstructured, error) {
										applyAttemptCount++
										if applyAttemptCount == 1 {
											return nil, k8serrors.NewTooManyRequestsError("server overloaded")
										}
										return obj, nil
									},
								}
							},
						}
					},
				},
			},
			want: want{
				err: nil, // Should succeed after retry
			},
		},
		"RetryOnApplyStatus": {
			args: args{
				resources:   []unstructured.Unstructured{testResource},
				applyStatus: true,
			},
			fields: fields{
				resourceMapper: &mockRESTMapper{
					mappingFunc: func(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
						return &meta.RESTMapping{
							Resource: schema.GroupVersionResource{
								Group:    "test.upbound.io",
								Version:  "v1",
								Resource: "testresources",
							},
						}, nil
					},
				},
				dynamicClient: &mockDynamicInterface{
					resourceFunc: func(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
						return &mockNamespaceableResourceInterface{
							namespaceFunc: func(ns string) dynamic.ResourceInterface {
								return &mockResourceInterface{
									applyFunc: func(ctx context.Context, name string, obj *unstructured.Unstructured, options v1.ApplyOptions, subresources ...string) (*unstructured.Unstructured, error) {
										return obj, nil
									},
									applyStatusFunc: func(ctx context.Context, name string, obj *unstructured.Unstructured, options v1.ApplyOptions) (*unstructured.Unstructured, error) {
										statusAttemptCount++
										if statusAttemptCount == 1 {
											return nil, k8serrors.NewConflict(schema.GroupResource{Group: "test.upbound.io", Resource: "testresources"}, name, errors.New("resource modified"))
										}
										return obj, nil
									},
								}
							},
						}
					},
				},
			},
			want: want{
				err: nil, // Should succeed after retry
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			a := &UnstructuredResourceApplier{
				dynamicClient:  tc.fields.dynamicClient,
				resourceMapper: tc.fields.resourceMapper,
			}

			got := a.ApplyResources(context.Background(), tc.args.resources, tc.args.applyStatus)

			if tc.want.err == nil && got != nil {
				t.Errorf("ApplyResources() error = %v, want nil", got)
			}

			if tc.want.err != nil && got == nil {
				t.Errorf("ApplyResources() error = nil, want %v", tc.want.err)
			}
		})
	}
}

func TestModifyResources(t *testing.T) {
	type args struct {
		resources []unstructured.Unstructured
		modify    func(*unstructured.Unstructured) error
	}

	type fields struct {
		dynamicClient  dynamic.Interface
		resourceMapper meta.RESTMapper
	}

	type want struct {
		err error
	}

	// Setup test resource
	testResource := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "test.upbound.io/v1",
			"kind":       "TestResource",
			"metadata": map[string]interface{}{
				"name":      "test-resource",
				"namespace": "test-namespace",
			},
			"spec": map[string]interface{}{
				"foo": "bar",
			},
		},
	}

	// Setup test error
	testError := errors.New("test error")

	// Reset counters for this test
	getAttemptCount := 0
	updateAttemptCount := 0

	cases := map[string]struct {
		args   args
		fields fields
		want   want
	}{
		"Success": {
			args: args{
				resources: []unstructured.Unstructured{testResource},
				modify: func(u *unstructured.Unstructured) error {
					return nil
				},
			},
			fields: fields{
				resourceMapper: &mockRESTMapper{
					mappingFunc: func(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
						return &meta.RESTMapping{
							Resource: schema.GroupVersionResource{
								Group:    "test.upbound.io",
								Version:  "v1",
								Resource: "testresources",
							},
						}, nil
					},
				},
				dynamicClient: &mockDynamicInterface{
					resourceFunc: func(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
						return &mockNamespaceableResourceInterface{
							namespaceFunc: func(ns string) dynamic.ResourceInterface {
								return &mockResourceInterface{
									getFunc: func(ctx context.Context, name string, options v1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
										return &testResource, nil
									},
									updateFunc: func(ctx context.Context, obj *unstructured.Unstructured, options v1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error) {
										return obj, nil
									},
								}
							},
						}
					},
				},
			},
			want: want{
				err: nil,
			},
		},
		"ErrorOnRESTMapping": {
			args: args{
				resources: []unstructured.Unstructured{testResource},
				modify: func(u *unstructured.Unstructured) error {
					return nil
				},
			},
			fields: fields{
				resourceMapper: &mockRESTMapper{
					mappingFunc: func(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
						return nil, testError
					},
				},
				dynamicClient: &mockDynamicInterface{},
			},
			want: want{
				err: testError,
			},
		},
		"ErrorOnGet": {
			args: args{
				resources: []unstructured.Unstructured{testResource},
				modify: func(u *unstructured.Unstructured) error {
					return nil
				},
			},
			fields: fields{
				resourceMapper: &mockRESTMapper{
					mappingFunc: func(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
						return &meta.RESTMapping{
							Resource: schema.GroupVersionResource{
								Group:    "test.upbound.io",
								Version:  "v1",
								Resource: "testresources",
							},
						}, nil
					},
				},
				dynamicClient: &mockDynamicInterface{
					resourceFunc: func(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
						return &mockNamespaceableResourceInterface{
							namespaceFunc: func(ns string) dynamic.ResourceInterface {
								return &mockResourceInterface{
									getFunc: func(ctx context.Context, name string, options v1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
										return nil, testError
									},
								}
							},
						}
					},
				},
			},
			want: want{
				err: testError,
			},
		},
		"ErrorOnModify": {
			args: args{
				resources: []unstructured.Unstructured{testResource},
				modify: func(u *unstructured.Unstructured) error {
					return testError
				},
			},
			fields: fields{
				resourceMapper: &mockRESTMapper{
					mappingFunc: func(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
						return &meta.RESTMapping{
							Resource: schema.GroupVersionResource{
								Group:    "test.upbound.io",
								Version:  "v1",
								Resource: "testresources",
							},
						}, nil
					},
				},
				dynamicClient: &mockDynamicInterface{
					resourceFunc: func(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
						return &mockNamespaceableResourceInterface{
							namespaceFunc: func(ns string) dynamic.ResourceInterface {
								return &mockResourceInterface{
									getFunc: func(ctx context.Context, name string, options v1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
										return &testResource, nil
									},
								}
							},
						}
					},
				},
			},
			want: want{
				err: testError,
			},
		},
		"ErrorOnUpdate": {
			args: args{
				resources: []unstructured.Unstructured{testResource},
				modify: func(u *unstructured.Unstructured) error {
					return nil
				},
			},
			fields: fields{
				resourceMapper: &mockRESTMapper{
					mappingFunc: func(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
						return &meta.RESTMapping{
							Resource: schema.GroupVersionResource{
								Group:    "test.upbound.io",
								Version:  "v1",
								Resource: "testresources",
							},
						}, nil
					},
				},
				dynamicClient: &mockDynamicInterface{
					resourceFunc: func(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
						return &mockNamespaceableResourceInterface{
							namespaceFunc: func(ns string) dynamic.ResourceInterface {
								return &mockResourceInterface{
									getFunc: func(ctx context.Context, name string, options v1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
										return &testResource, nil
									},
									updateFunc: func(ctx context.Context, obj *unstructured.Unstructured, options v1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error) {
										return nil, testError
									},
								}
							},
						}
					},
				},
			},
			want: want{
				err: testError,
			},
		},
		"RetryOnGetResource": {
			args: args{
				resources: []unstructured.Unstructured{testResource},
				modify: func(u *unstructured.Unstructured) error {
					return nil
				},
			},
			fields: fields{
				resourceMapper: &mockRESTMapper{
					mappingFunc: func(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
						return &meta.RESTMapping{
							Resource: schema.GroupVersionResource{
								Group:    "test.upbound.io",
								Version:  "v1",
								Resource: "testresources",
							},
						}, nil
					},
				},
				dynamicClient: &mockDynamicInterface{
					resourceFunc: func(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
						return &mockNamespaceableResourceInterface{
							namespaceFunc: func(ns string) dynamic.ResourceInterface {
								return &mockResourceInterface{
									getFunc: func(ctx context.Context, name string, options v1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
										getAttemptCount++
										if getAttemptCount == 1 {
											return nil, k8serrors.NewTooManyRequestsError("server overloaded")
										}
										return &testResource, nil
									},
									updateFunc: func(ctx context.Context, obj *unstructured.Unstructured, options v1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error) {
										return obj, nil
									},
								}
							},
						}
					},
				},
			},
			want: want{
				err: nil, // Should succeed after retry
			},
		},
		"RetryOnUpdateResource": {
			args: args{
				resources: []unstructured.Unstructured{testResource},
				modify: func(u *unstructured.Unstructured) error {
					return nil
				},
			},
			fields: fields{
				resourceMapper: &mockRESTMapper{
					mappingFunc: func(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
						return &meta.RESTMapping{
							Resource: schema.GroupVersionResource{
								Group:    "test.upbound.io",
								Version:  "v1",
								Resource: "testresources",
							},
						}, nil
					},
				},
				dynamicClient: &mockDynamicInterface{
					resourceFunc: func(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
						return &mockNamespaceableResourceInterface{
							namespaceFunc: func(ns string) dynamic.ResourceInterface {
								return &mockResourceInterface{
									getFunc: func(ctx context.Context, name string, options v1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
										// Get succeeds immediately
										return &testResource, nil
									},
									updateFunc: func(ctx context.Context, obj *unstructured.Unstructured, options v1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error) {
										updateAttemptCount++
										if updateAttemptCount == 1 {
											// First attempt fails with a Kubernetes API error
											return nil, k8serrors.NewServerTimeout(schema.GroupResource{Group: "test.upbound.io", Resource: "testresources"}, "update", 5)
										}
										// Second attempt succeeds
										return obj, nil
									},
								}
							},
						}
					},
				},
			},
			want: want{
				err: nil, // Should succeed after retry
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			a := &UnstructuredResourceApplier{
				dynamicClient:  tc.fields.dynamicClient,
				resourceMapper: tc.fields.resourceMapper,
			}

			got := a.ModifyResources(context.Background(), tc.args.resources, tc.args.modify)

			if tc.want.err == nil && got != nil {
				t.Errorf("ModifyResources() error = %v, want nil", got)
			}

			if tc.want.err != nil && got == nil {
				t.Errorf("ModifyResources() error = nil, want %v", tc.want.err)
			}
		})
	}
}

func TestNewUnstructuredResourceApplier(t *testing.T) {
	// Given
	scheme := runtime.NewScheme()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)
	restMapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{})

	// When
	ra := NewUnstructuredResourceApplier(dynamicClient, restMapper)

	// Then
	if ra.dynamicClient != dynamicClient {
		t.Errorf("Expected dynamicClient to be %v, got %v", dynamicClient, ra.dynamicClient)
	}

	if ra.resourceMapper != restMapper {
		t.Errorf("Expected resourceMapper to be %v, got %v", restMapper, ra.resourceMapper)
	}
}

// mockRESTMapper is a mock implementation of RESTMapper
type mockRESTMapper struct {
	meta.RESTMapper
	mappingFunc func(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error)
}

func (m *mockRESTMapper) RESTMapping(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
	return m.mappingFunc(gk, versions...)
}

// mockDynamicInterface is a mock implementation of dynamic.Interface
type mockDynamicInterface struct {
	dynamic.Interface
	resourceFunc func(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface
}

func (m *mockDynamicInterface) Resource(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return m.resourceFunc(resource)
}

// mockNamespaceableResourceInterface is a mock implementation of dynamic.NamespaceableResourceInterface
type mockNamespaceableResourceInterface struct {
	dynamic.NamespaceableResourceInterface
	namespaceFunc func(ns string) dynamic.ResourceInterface
}

func (m *mockNamespaceableResourceInterface) Namespace(ns string) dynamic.ResourceInterface {
	return m.namespaceFunc(ns)
}

// mockResourceInterface is a mock implementation of dynamic.ResourceInterface
type mockResourceInterface struct {
	dynamic.ResourceInterface
	applyFunc       func(ctx context.Context, name string, obj *unstructured.Unstructured, options v1.ApplyOptions, subresources ...string) (*unstructured.Unstructured, error)
	applyStatusFunc func(ctx context.Context, name string, obj *unstructured.Unstructured, options v1.ApplyOptions) (*unstructured.Unstructured, error)
	getFunc         func(ctx context.Context, name string, options v1.GetOptions, subresources ...string) (*unstructured.Unstructured, error)
	updateFunc      func(ctx context.Context, obj *unstructured.Unstructured, options v1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error)
}

func (m *mockResourceInterface) Apply(ctx context.Context, name string, obj *unstructured.Unstructured, options v1.ApplyOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return m.applyFunc(ctx, name, obj, options, subresources...)
}

func (m *mockResourceInterface) ApplyStatus(ctx context.Context, name string, obj *unstructured.Unstructured, options v1.ApplyOptions) (*unstructured.Unstructured, error) {
	return m.applyStatusFunc(ctx, name, obj, options)
}

func (m *mockResourceInterface) Get(ctx context.Context, name string, options v1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return m.getFunc(ctx, name, options, subresources...)
}

func (m *mockResourceInterface) Update(ctx context.Context, obj *unstructured.Unstructured, options v1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return m.updateFunc(ctx, obj, options, subresources...)
}
