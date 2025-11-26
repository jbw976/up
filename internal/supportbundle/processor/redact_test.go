// Copyright 2025 Upbound Inc.
// All rights reserved

package processor

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestRedactConfigMaps(t *testing.T) {
	tests := map[string]struct {
		setup   func(bundleRoot string) error
		wantErr error
		verify  bool
	}{
		"RedactConfigMapList": {
			setup: func(bundleRoot string) error {
				configMapsDir := filepath.Join(bundleRoot, "cluster-resources", "configmaps")
				_ = os.MkdirAll(configMapsDir, 0o755)

				cmList := &corev1.ConfigMapList{
					Items: []corev1.ConfigMap{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "cm1",
							},
							Data: map[string]string{
								"key1": "value1",
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "cm2",
							},
							Data: map[string]string{
								"key2": "value2",
							},
						},
					},
				}
				data, err := json.Marshal(cmList)
				if err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(configMapsDir, "configmaps.json"), data, 0o600)
			},
			wantErr: nil,
			verify:  true,
		},
		"NoClusterResourcesDirectory": {
			setup: func(_ string) error {
				return nil
			},
			wantErr: nil,
			verify:  false,
		},
		"NoConfigMapsDirectory": {
			setup: func(bundleRoot string) error {
				_ = os.MkdirAll(filepath.Join(bundleRoot, "cluster-resources"), 0o755)
				return nil
			},
			wantErr: nil,
			verify:  false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			bundleRoot := t.TempDir()
			if err := tt.setup(bundleRoot); err != nil {
				t.Fatalf("setup failed: %v", err)
			}

			err := RedactConfigMaps(context.Background(), bundleRoot)

			if diff := cmp.Diff(tt.wantErr, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("redactConfigMaps(...): -want err, +got err\n%s", diff)
			}
			if tt.verify {
				configMapsDir := filepath.Join(bundleRoot, "cluster-resources", "configmaps")
				filePath := filepath.Join(configMapsDir, "configmaps.json")
				data, err := os.ReadFile(filePath)
				if err != nil {
					t.Fatalf("failed to read file: %v", err)
				}

				var got corev1.ConfigMapList
				if err := json.Unmarshal(data, &got); err != nil {
					t.Fatalf("failed to unmarshal: %v", err)
				}

				for i, cm := range got.Items {
					if len(cm.Data) != 0 {
						t.Errorf("item %d: data field should be empty, got %d keys", i, len(cm.Data))
					}
				}
			}
		})
	}
}

func TestRedactEnvironmentConfigs(t *testing.T) {
	tests := map[string]struct {
		setup   func(bundleRoot string) error
		wantErr error
		verify  bool
	}{
		"RedactEnvironmentConfigArray": {
			setup: func(bundleRoot string) error {
				customResourcesDir := filepath.Join(bundleRoot, "cluster-resources", "custom-resources")
				_ = os.MkdirAll(customResourcesDir, 0o755)

				configs := []map[string]any{
					{
						"kind": "EnvironmentConfig",
						"metadata": map[string]any{
							"name": "env1",
							"annotations": map[string]any{
								"kubectl.kubernetes.io/last-applied-configuration": "some-config",
							},
						},
						"data": map[string]any{
							"key1": "value1",
						},
					},
					{
						"kind": "EnvironmentConfig",
						"metadata": map[string]any{
							"name": "env2",
						},
						"data": map[string]any{
							"key2": "value2",
						},
					},
				}
				data, err := json.Marshal(configs)
				if err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(customResourcesDir, "environmentconfigs.apiextensions.crossplane.io.json"), data, 0o600)
			},
			wantErr: nil,
			verify:  true,
		},
		"RedactSingleEnvironmentConfig": {
			setup: func(bundleRoot string) error {
				customResourcesDir := filepath.Join(bundleRoot, "cluster-resources", "custom-resources")
				_ = os.MkdirAll(customResourcesDir, 0o755)

				configs := []map[string]any{
					{
						"kind": "EnvironmentConfig",
						"metadata": map[string]any{
							"name": "env1",
							"annotations": map[string]any{
								"kubectl.kubernetes.io/last-applied-configuration": "some-config",
							},
						},
						"data": map[string]any{
							"key1": "value1",
						},
					},
				}
				data, err := json.Marshal(configs)
				if err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(customResourcesDir, "environmentconfigs.apiextensions.crossplane.io.json"), data, 0o600)
			},
			wantErr: nil,
			verify:  true,
		},
		"NoClusterResourcesDirectory": {
			setup: func(_ string) error {
				return nil
			},
			wantErr: nil,
			verify:  false,
		},
		"NoEnvironmentConfigFile": {
			setup: func(bundleRoot string) error {
				customResourcesDir := filepath.Join(bundleRoot, "cluster-resources", "custom-resources")
				_ = os.MkdirAll(customResourcesDir, 0o755)
				return nil
			},
			wantErr: nil,
			verify:  false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			bundleRoot := t.TempDir()
			if err := tt.setup(bundleRoot); err != nil {
				t.Fatalf("setup failed: %v", err)
			}

			err := RedactEnvironmentConfigs(context.Background(), bundleRoot)

			if diff := cmp.Diff(tt.wantErr, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("redactEnvironmentConfigs(...): -want err, +got err\n%s", diff)
			}
			if tt.verify {
				customResourcesDir := filepath.Join(bundleRoot, "cluster-resources", "custom-resources")
				data, err := os.ReadFile(filepath.Join(customResourcesDir, "environmentconfigs.apiextensions.crossplane.io.json"))
				if err != nil {
					t.Fatalf("failed to read file: %v", err)
				}

				var configs []unstructured.Unstructured
				if err := json.Unmarshal(data, &configs); err != nil {
					t.Fatalf("failed to unmarshal: %v", err)
				}

				for i := range configs {
					dataField, found, _ := unstructured.NestedMap(configs[i].Object, "data")
					if !found {
						t.Fatalf("item %d: data field should exist", i)
					}
					if len(dataField) != 0 {
						t.Errorf("item %d: data field should be empty, got %d keys", i, len(dataField))
					}
					checkObjectAnnotationRemoved(t, configs[i], i)
				}
			}
		})
	}
}

func TestRedactProviderKubernetesObjects(t *testing.T) {
	tests := map[string]struct {
		setup   func(bundleRoot string) error
		wantErr error
		verify  bool
	}{
		"RedactSecretInSpec": {
			setup: func(bundleRoot string) error {
				customResourcesDir := filepath.Join(bundleRoot, "cluster-resources", "custom-resources")
				_ = os.MkdirAll(customResourcesDir, 0o755)

				objects := []map[string]any{
					{
						"apiVersion": "kubernetes.crossplane.io/v1alpha2",
						"kind":       "Object",
						"metadata": map[string]any{
							"name": "example-secret",
							"annotations": map[string]any{
								"kubectl.kubernetes.io/last-applied-configuration": "{\"data\":{\"password\":\"secret\"}}",
							},
						},
						"spec": map[string]any{
							"forProvider": map[string]any{
								"manifest": map[string]any{
									"kind": "Secret",
									"data": map[string]any{
										"password": "cGFzc3dvcmQ=",
										"username": "dXNlcm5hbWU=",
									},
								},
							},
						},
						"status": map[string]any{
							"atProvider": map[string]any{
								"manifest": map[string]any{
									"kind": "Secret",
									"data": map[string]any{
										"password": "cGFzc3dvcmQ=",
										"username": "dXNlcm5hbWU=",
									},
									"metadata": map[string]any{
										"annotations": map[string]any{
											"kubectl.kubernetes.io/last-applied-configuration": "{\"data\":{\"password\":\"secret\"}}",
										},
									},
								},
							},
						},
					},
				}
				data, err := json.Marshal(objects)
				if err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(customResourcesDir, "objects.kubernetes.crossplane.io.json"), data, 0o600)
			},
			wantErr: nil,
			verify:  true,
		},
		"NoObjectsFile": {
			setup: func(bundleRoot string) error {
				customResourcesDir := filepath.Join(bundleRoot, "cluster-resources", "custom-resources")
				_ = os.MkdirAll(customResourcesDir, 0o755)
				return nil
			},
			wantErr: nil,
			verify:  false,
		},
		"ObjectWithNonSecretManifest": {
			setup: func(bundleRoot string) error {
				customResourcesDir := filepath.Join(bundleRoot, "cluster-resources", "custom-resources")
				_ = os.MkdirAll(customResourcesDir, 0o755)

				objects := []map[string]any{
					{
						"apiVersion": "kubernetes.crossplane.io/v1alpha2",
						"kind":       "Object",
						"spec": map[string]any{
							"forProvider": map[string]any{
								"manifest": map[string]any{
									"kind": "SomeOtherKind",
									"data": map[string]any{
										"key": "value",
									},
								},
							},
						},
					},
				}
				data, err := json.Marshal(objects)
				if err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(customResourcesDir, "objects.kubernetes.crossplane.io.json"), data, 0o600)
			},
			wantErr: nil,
			verify:  false,
		},
		"RedactSecretWithStringData": {
			setup: func(bundleRoot string) error {
				customResourcesDir := filepath.Join(bundleRoot, "cluster-resources", "custom-resources")
				_ = os.MkdirAll(customResourcesDir, 0o755)

				objects := []map[string]any{
					{
						"apiVersion": "kubernetes.crossplane.io/v1alpha2",
						"kind":       "Object",
						"metadata": map[string]any{
							"name": "example-secret-with-stringdata",
							"annotations": map[string]any{
								"kubectl.kubernetes.io/last-applied-configuration": "{\"stringData\":{\"password\":\"secret\"}}",
							},
						},
						"spec": map[string]any{
							"forProvider": map[string]any{
								"manifest": map[string]any{
									"kind": "Secret",
									"data": map[string]any{
										"password": "cGFzc3dvcmQ=",
									},
									"stringData": map[string]any{
										"apiKey":    "my-api-key",
										"authToken": "my-auth-token",
									},
								},
							},
						},
						"status": map[string]any{
							"atProvider": map[string]any{
								"manifest": map[string]any{
									"kind": "Secret",
									"data": map[string]any{
										"password": "cGFzc3dvcmQ=",
									},
									"stringData": map[string]any{
										"apiKey":    "my-api-key",
										"authToken": "my-auth-token",
									},
									"metadata": map[string]any{
										"annotations": map[string]any{
											"kubectl.kubernetes.io/last-applied-configuration": "{\"stringData\":{\"password\":\"secret\"}}",
										},
									},
								},
							},
						},
					},
				}
				data, err := json.Marshal(objects)
				if err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(customResourcesDir, "objects.kubernetes.crossplane.io.json"), data, 0o600)
			},
			wantErr: nil,
			verify:  true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			bundleRoot := t.TempDir()
			if err := tt.setup(bundleRoot); err != nil {
				t.Fatalf("setup failed: %v", err)
			}

			err := RedactProviderKubernetesObjects(context.Background(), bundleRoot)

			if diff := cmp.Diff(tt.wantErr, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("redactProviderKubernetesObjects(...): -want err, +got err\n%s", diff)
			}
			if tt.verify {
				customResourcesDir := filepath.Join(bundleRoot, "cluster-resources", "custom-resources")
				data, err := os.ReadFile(filepath.Join(customResourcesDir, "objects.kubernetes.crossplane.io.json"))
				if err != nil {
					t.Fatalf("failed to read file: %v", err)
				}

				var objects []unstructured.Unstructured
				if err := json.Unmarshal(data, &objects); err != nil {
					t.Fatalf("failed to unmarshal: %v", err)
				}

				for i := range objects {
					checkSecretDataEmpty(t, objects[i], []string{"spec", "forProvider", "manifest"}, i)
					checkSecretStringDataEmpty(t, objects[i], []string{"spec", "forProvider", "manifest"}, i)
					checkSecretDataEmpty(t, objects[i], []string{"status", "atProvider", "manifest"}, i)
					checkSecretStringDataEmpty(t, objects[i], []string{"status", "atProvider", "manifest"}, i)
					checkAnnotationRemoved(t, objects[i], []string{"status", "atProvider", "manifest", "metadata"}, i)
					checkObjectAnnotationRemoved(t, objects[i], i)
				}
			}
		})
	}
}

func checkSecretDataEmpty(t *testing.T, obj unstructured.Unstructured, path []string, itemIdx int) {
	manifest, found, _ := unstructured.NestedMap(obj.Object, path...)
	if !found {
		return
	}
	kind, _, _ := unstructured.NestedString(manifest, "kind")
	if kind != "Secret" {
		return
	}
	dataField, _, _ := unstructured.NestedMap(manifest, "data")
	if len(dataField) != 0 {
		t.Errorf("item %d: %v.data should be empty for Secret, got %d keys", itemIdx, path, len(dataField))
	}
}

func checkSecretStringDataEmpty(t *testing.T, obj unstructured.Unstructured, path []string, itemIdx int) {
	manifest, found, _ := unstructured.NestedMap(obj.Object, path...)
	if !found {
		return
	}
	kind, _, _ := unstructured.NestedString(manifest, "kind")
	if kind != "Secret" {
		return
	}
	stringDataField, _, _ := unstructured.NestedMap(manifest, "stringData")
	if len(stringDataField) != 0 {
		t.Errorf("item %d: %v.stringData should be empty for Secret, got %d keys", itemIdx, path, len(stringDataField))
	}
}

func checkAnnotationRemoved(t *testing.T, obj unstructured.Unstructured, path []string, itemIdx int) {
	metadata, found, _ := unstructured.NestedMap(obj.Object, path...)
	if !found {
		return
	}
	annotations, found, _ := unstructured.NestedMap(metadata, "annotations")
	if !found {
		return
	}
	if _, has := annotations["kubectl.kubernetes.io/last-applied-configuration"]; has {
		t.Errorf("item %d: kubectl last-applied-configuration should be removed from %v", itemIdx, path)
	}
}

func checkObjectAnnotationRemoved(t *testing.T, obj unstructured.Unstructured, itemIdx int) {
	annotations := obj.GetAnnotations()
	if annotations != nil {
		if _, has := annotations["kubectl.kubernetes.io/last-applied-configuration"]; has {
			t.Errorf("item %d: kubectl last-applied-configuration should be removed from Object metadata", itemIdx)
		}
	}
}
