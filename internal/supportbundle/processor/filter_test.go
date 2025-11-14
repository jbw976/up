// Copyright 2025 Upbound Inc.
// All rights reserved

package processor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_isCrossplaneCRD(t *testing.T) {
	tests := []struct {
		name string
		crd  apiextensionsv1.CustomResourceDefinition
		want bool
	}{
		{
			name: "CrossplaneAPIGroup",
			crd: apiextensionsv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: "compositeresourcedefinitions.apiextensions.crossplane.io",
				},
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Group: "apiextensions.crossplane.io",
					Names: apiextensionsv1.CustomResourceDefinitionNames{
						Categories: []string{},
					},
				},
			},
			want: true,
		},
		{
			name: "UpboundAPIGroup",
			crd: apiextensionsv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: "providers.pkg.upbound.io",
				},
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Group: "pkg.upbound.io",
					Names: apiextensionsv1.CustomResourceDefinitionNames{
						Categories: []string{},
					},
				},
			},
			want: true,
		},
		{
			name: "CrossplaneCategory",
			crd: apiextensionsv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: "some.resource.io",
				},
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Group: "resource.io",
					Names: apiextensionsv1.CustomResourceDefinitionNames{
						Categories: []string{"crossplane"},
					},
				},
			},
			want: true,
		},
		{
			name: "CompositesCategory",
			crd: apiextensionsv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: "some.resource.io",
				},
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Group: "resource.io",
					Names: apiextensionsv1.CustomResourceDefinitionNames{
						Categories: []string{"composites"},
					},
				},
			},
			want: true,
		},
		{
			name: "ManagedCategory",
			crd: apiextensionsv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: "some.resource.io",
				},
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Group: "resource.io",
					Names: apiextensionsv1.CustomResourceDefinitionNames{
						Categories: []string{"managed"},
					},
				},
			},
			want: true,
		},
		{
			name: "NonCrossplaneCRD",
			crd: apiextensionsv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: "externalsecrets.external-secrets.io",
				},
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Group: "external-secrets.io",
					Names: apiextensionsv1.CustomResourceDefinitionNames{
						Categories: []string{"external-secrets"},
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isCrossplaneCRD(tt.crd)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("%s\nisCrossplaneCRD(...): -want, +got\n%s", tt.name, diff)
			}
		})
	}
}

func Test_filterCRDArrayFile(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(dir string) string
		wantKept map[string]bool
		wantErr  error
	}{
		{
			name: "FilterMixedCRDs",
			setup: func(dir string) string {
				crdList := apiextensionsv1.CustomResourceDefinitionList{
					TypeMeta: metav1.TypeMeta{
						Kind:       "CustomResourceDefinitionList",
						APIVersion: "apiextensions.k8s.io/v1",
					},
					ListMeta: metav1.ListMeta{
						ResourceVersion: "12345",
					},
					Items: []apiextensionsv1.CustomResourceDefinition{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "compositeresourcedefinitions.apiextensions.crossplane.io",
							},
							Spec: apiextensionsv1.CustomResourceDefinitionSpec{
								Group: "apiextensions.crossplane.io",
								Names: apiextensionsv1.CustomResourceDefinitionNames{
									Categories: []string{},
								},
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "externalsecrets.external-secrets.io",
							},
							Spec: apiextensionsv1.CustomResourceDefinitionSpec{
								Group: "external-secrets.io",
								Names: apiextensionsv1.CustomResourceDefinitionNames{
									Categories: []string{},
								},
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "providers.pkg.upbound.io",
							},
							Spec: apiextensionsv1.CustomResourceDefinitionSpec{
								Group: "pkg.upbound.io",
								Names: apiextensionsv1.CustomResourceDefinitionNames{
									Categories: []string{},
								},
							},
						},
					},
				}
				data, err := json.Marshal(crdList)
				if err != nil {
					return ""
				}
				filePath := filepath.Join(dir, "custom-resource-definitions.json")
				_ = os.WriteFile(filePath, data, 0o600)
				return filePath
			},
			wantKept: map[string]bool{
				"compositeresourcedefinitions.apiextensions.crossplane.io": true,
				"providers.pkg.upbound.io":                                 true,
			},
			wantErr: nil,
		},
		{
			name: "EmptyList",
			setup: func(dir string) string {
				crdList := apiextensionsv1.CustomResourceDefinitionList{
					TypeMeta: metav1.TypeMeta{
						Kind:       "CustomResourceDefinitionList",
						APIVersion: "apiextensions.k8s.io/v1",
					},
					ListMeta: metav1.ListMeta{
						ResourceVersion: "12345",
					},
					Items: []apiextensionsv1.CustomResourceDefinition{},
				}
				data, err := json.Marshal(crdList)
				if err != nil {
					return ""
				}
				filePath := filepath.Join(dir, "custom-resource-definitions.json")
				_ = os.WriteFile(filePath, data, 0o600)
				return filePath
			},
			wantKept: map[string]bool{},
			wantErr:  nil,
		},
		{
			name: "InvalidJSON",
			setup: func(dir string) string {
				filePath := filepath.Join(dir, "custom-resource-definitions.json")
				_ = os.WriteFile(filePath, []byte("invalid json"), 0o600)
				return filePath
			},
			wantKept: nil,
			wantErr:  cmpopts.AnyError,
		},
		{
			name: "PreservesMetadata",
			setup: func(dir string) string {
				crdList := apiextensionsv1.CustomResourceDefinitionList{
					TypeMeta: metav1.TypeMeta{
						Kind:       "CustomResourceDefinitionList",
						APIVersion: "apiextensions.k8s.io/v1",
					},
					ListMeta: metav1.ListMeta{
						ResourceVersion: "99999",
					},
					Items: []apiextensionsv1.CustomResourceDefinition{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "providers.pkg.upbound.io",
							},
							Spec: apiextensionsv1.CustomResourceDefinitionSpec{
								Group: "pkg.upbound.io",
								Names: apiextensionsv1.CustomResourceDefinitionNames{
									Categories: []string{},
								},
							},
						},
					},
				}
				data, err := json.Marshal(crdList)
				if err != nil {
					return ""
				}
				filePath := filepath.Join(dir, "custom-resource-definitions.json")
				_ = os.WriteFile(filePath, data, 0o600)
				return filePath
			},
			wantKept: map[string]bool{
				"providers.pkg.upbound.io": true,
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			filePath := tt.setup(dir)

			kept, err := filterCRDList(filePath)

			if diff := cmp.Diff(tt.wantErr, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nfilterCRDList(...): -want err, +got err\n%s", tt.name, diff)
			}

			if diff := cmp.Diff(tt.wantKept, kept); diff != "" {
				t.Errorf("%s\nfilterCRDList(...): unexpected kept CRDs (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func Test_filterCustomResources(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(dir string) (string, map[string]bool)
		wantKept    []string
		wantRemoved []string
		wantErr     error
	}{
		{
			name: "KeepCrossplaneResources",
			setup: func(dir string) (string, map[string]bool) {
				customResourcesDir := filepath.Join(dir, "custom-resources")
				_ = os.MkdirAll(customResourcesDir, 0o755)

				_ = os.WriteFile(filepath.Join(customResourcesDir, "objects.kubernetes.crossplane.io.json"), []byte(`[]`), 0o600)
				_ = os.WriteFile(filepath.Join(customResourcesDir, "objects.kubernetes.crossplane.io.yaml"), []byte(`[]`), 0o644)
				_ = os.WriteFile(filepath.Join(customResourcesDir, "providers.pkg.crossplane.io.json"), []byte(`[]`), 0o644)
				_ = os.WriteFile(filepath.Join(customResourcesDir, "externalsecrets.external-secrets.io.json"), []byte(`[]`), 0o644)
				_ = os.WriteFile(filepath.Join(customResourcesDir, "clusterissuers.cert-manager.io.json"), []byte(`[]`), 0o644)

				keptCRDNames := map[string]bool{
					"objects.kubernetes.crossplane.io": true,
					"providers.pkg.crossplane.io":      true,
				}

				return customResourcesDir, keptCRDNames
			},
			wantKept: []string{
				"objects.kubernetes.crossplane.io.json",
				"objects.kubernetes.crossplane.io.yaml",
				"providers.pkg.crossplane.io.json",
			},
			wantRemoved: []string{
				"externalsecrets.external-secrets.io.json",
				"clusterissuers.cert-manager.io.json",
			},
			wantErr: nil,
		},
		{
			name: "KeepAllWhenAllCrossplane",
			setup: func(dir string) (string, map[string]bool) {
				customResourcesDir := filepath.Join(dir, "custom-resources")
				_ = os.MkdirAll(customResourcesDir, 0o755)

				_ = os.WriteFile(filepath.Join(customResourcesDir, "objects.kubernetes.crossplane.io.json"), []byte(`[]`), 0o600)
				_ = os.WriteFile(filepath.Join(customResourcesDir, "providers.pkg.crossplane.io.json"), []byte(`[]`), 0o644)

				keptCRDNames := map[string]bool{
					"objects.kubernetes.crossplane.io": true,
					"providers.pkg.crossplane.io":      true,
				}

				return customResourcesDir, keptCRDNames
			},
			wantKept: []string{
				"objects.kubernetes.crossplane.io.json",
				"providers.pkg.crossplane.io.json",
			},
			wantRemoved: []string{},
			wantErr:     nil,
		},
		{
			name: "RemoveAllWhenNoneCrossplane",
			setup: func(dir string) (string, map[string]bool) {
				customResourcesDir := filepath.Join(dir, "custom-resources")
				_ = os.MkdirAll(customResourcesDir, 0o755)

				_ = os.WriteFile(filepath.Join(customResourcesDir, "externalsecrets.external-secrets.io.json"), []byte(`[]`), 0o644)
				_ = os.WriteFile(filepath.Join(customResourcesDir, "clusterissuers.cert-manager.io.json"), []byte(`[]`), 0o644)

				keptCRDNames := map[string]bool{}

				return customResourcesDir, keptCRDNames
			},
			wantKept: []string{},
			wantRemoved: []string{
				"externalsecrets.external-secrets.io.json",
				"clusterissuers.cert-manager.io.json",
			},
			wantErr: nil,
		},
		{
			name: "HandleMissingDirectory",
			setup: func(dir string) (string, map[string]bool) {
				customResourcesDir := filepath.Join(dir, "custom-resources")
				keptCRDNames := map[string]bool{}
				return customResourcesDir, keptCRDNames
			},
			wantKept:    []string{},
			wantRemoved: []string{},
			wantErr:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			customResourcesDir, keptCRDNames := tt.setup(dir)

			err := filterCustomResources(customResourcesDir, keptCRDNames)

			if diff := cmp.Diff(tt.wantErr, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nfilterCustomResources(...): -want err, +got err\n%s", tt.name, diff)
			}

			for _, fileName := range tt.wantKept {
				filePath := filepath.Join(customResourcesDir, fileName)
				_, err := os.Stat(filePath)
				if err != nil {
					t.Errorf("%s\nexpected file %q to exist: %v", tt.name, fileName, err)
				}
			}

			for _, fileName := range tt.wantRemoved {
				filePath := filepath.Join(customResourcesDir, fileName)
				_, err := os.Stat(filePath)
				if !os.IsNotExist(err) {
					t.Errorf("%s\nexpected file %q to be removed, but it exists", tt.name, fileName)
				}
			}
		})
	}
}

func Test_removeNonCrossplaneClusterResources(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(dir string) string
		wantKept    []string
		wantRemoved []string
		wantErr     error
	}{
		{
			name: "KeepEssentialFiles",
			setup: func(dir string) string {
				clusterResourcesDir := filepath.Join(dir, "cluster-resources")
				_ = os.MkdirAll(clusterResourcesDir, 0o755)

				_ = os.WriteFile(filepath.Join(clusterResourcesDir, "custom-resource-definitions.json"), []byte(`[]`), 0o600)
				_ = os.WriteFile(filepath.Join(clusterResourcesDir, "namespaces.json"), []byte(`[]`), 0o600)
				_ = os.WriteFile(filepath.Join(clusterResourcesDir, "resources.json"), []byte(`[]`), 0o600)

				_ = os.MkdirAll(filepath.Join(clusterResourcesDir, "custom-resources"), 0o755)

				_ = os.WriteFile(filepath.Join(clusterResourcesDir, "pods.json"), []byte(`[]`), 0o600)
				_ = os.WriteFile(filepath.Join(clusterResourcesDir, "services.json"), []byte(`[]`), 0o600)

				_ = os.MkdirAll(filepath.Join(clusterResourcesDir, "non-essential"), 0o755)
				_ = os.WriteFile(filepath.Join(clusterResourcesDir, "non-essential", "file.json"), []byte(`[]`), 0o600)

				return clusterResourcesDir
			},
			wantKept: []string{
				"custom-resource-definitions.json",
				"custom-resources",
				"namespaces.json",
				"resources.json",
			},
			wantRemoved: []string{
				"pods.json",
				"services.json",
				"non-essential",
			},
			wantErr: nil,
		},
		{
			name: "RemoveAllNonEssential",
			setup: func(dir string) string {
				clusterResourcesDir := filepath.Join(dir, "cluster-resources")
				_ = os.MkdirAll(clusterResourcesDir, 0o755)

				_ = os.WriteFile(filepath.Join(clusterResourcesDir, "custom-resource-definitions.json"), []byte(`[]`), 0o600)
				_ = os.MkdirAll(filepath.Join(clusterResourcesDir, "custom-resources"), 0o755)

				return clusterResourcesDir
			},
			wantKept: []string{
				"custom-resource-definitions.json",
				"custom-resources",
			},
			wantRemoved: []string{},
			wantErr:     nil,
		},
		{
			name: "HandleMissingDirectory",
			setup: func(dir string) string {
				clusterResourcesDir := filepath.Join(dir, "cluster-resources")
				return clusterResourcesDir
			},
			wantKept:    []string{},
			wantRemoved: []string{},
			wantErr:     cmpopts.AnyError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			clusterResourcesDir := tt.setup(dir)

			err := removeNonCrossplaneClusterResources(clusterResourcesDir)

			if diff := cmp.Diff(tt.wantErr, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nremoveNonCrossplaneClusterResources(...): -want err, +got err\n%s", tt.name, diff)
			}

			for _, itemName := range tt.wantKept {
				itemPath := filepath.Join(clusterResourcesDir, itemName)
				_, err := os.Stat(itemPath)
				if err != nil {
					t.Errorf("%s\nexpected item %q to exist: %v", tt.name, itemName, err)
				}
			}

			for _, itemName := range tt.wantRemoved {
				itemPath := filepath.Join(clusterResourcesDir, itemName)
				_, err := os.Stat(itemPath)
				if !os.IsNotExist(err) {
					t.Errorf("%s\nexpected item %q to be removed, but it exists", tt.name, itemName)
				}
			}
		})
	}
}

func Test_removeNonEssentialBundleDirectories(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(dir string) string
		wantKept    []string
		wantRemoved []string
		wantErr     error
	}{
		{
			name: "KeepEssentialDirectories",
			setup: func(dir string) string {
				bundleRoot := filepath.Join(dir, "bundle")
				_ = os.MkdirAll(bundleRoot, 0o755)

				_ = os.MkdirAll(filepath.Join(bundleRoot, "cluster-resources"), 0o755)
				_ = os.MkdirAll(filepath.Join(bundleRoot, "execution-data"), 0o755)
				_ = os.MkdirAll(filepath.Join(bundleRoot, "cluster-info"), 0o755)

				_ = os.MkdirAll(filepath.Join(bundleRoot, "logs"), 0o755)
				_ = os.MkdirAll(filepath.Join(bundleRoot, "events"), 0o755)
				_ = os.WriteFile(filepath.Join(bundleRoot, "logs", "app.log"), []byte("log"), 0o600)

				_ = os.WriteFile(filepath.Join(bundleRoot, "metadata.json"), []byte(`{}`), 0o600)

				return bundleRoot
			},
			wantKept: []string{
				"cluster-resources",
				"execution-data",
				"cluster-info",
			},
			wantRemoved: []string{
				"logs",
				"events",
			},
			wantErr: nil,
		},
		{
			name: "RemoveAllNonEssential",
			setup: func(dir string) string {
				bundleRoot := filepath.Join(dir, "bundle")
				_ = os.MkdirAll(bundleRoot, 0o755)

				_ = os.MkdirAll(filepath.Join(bundleRoot, "cluster-resources"), 0o755)
				_ = os.MkdirAll(filepath.Join(bundleRoot, "execution-data"), 0o755)

				return bundleRoot
			},
			wantKept: []string{
				"cluster-resources",
				"execution-data",
			},
			wantRemoved: []string{},
			wantErr:     nil,
		},
		{
			name: "HandleMissingDirectory",
			setup: func(dir string) string {
				bundleRoot := filepath.Join(dir, "bundle")
				return bundleRoot
			},
			wantKept:    []string{},
			wantRemoved: []string{},
			wantErr:     cmpopts.AnyError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			bundleRoot := tt.setup(dir)

			err := removeNonEssentialBundleDirectories(bundleRoot)

			if diff := cmp.Diff(tt.wantErr, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nremoveNonEssentialBundleDirectories(...): -want err, +got err\n%s", tt.name, diff)
			}

			for _, dirName := range tt.wantKept {
				dirPath := filepath.Join(bundleRoot, dirName)
				info, err := os.Stat(dirPath)
				if err != nil {
					t.Errorf("%s\nexpected directory %q to exist: %v", tt.name, dirName, err)
				}
				if !info.IsDir() {
					t.Errorf("%s\nexpected %q to be a directory", tt.name, dirName)
				}
			}

			for _, dirName := range tt.wantRemoved {
				dirPath := filepath.Join(bundleRoot, dirName)
				_, err := os.Stat(dirPath)
				if !os.IsNotExist(err) {
					t.Errorf("%s\nexpected directory %q to be removed, but it exists", tt.name, dirName)
				}
			}
		})
	}
}
