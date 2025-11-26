// Copyright 2025 Upbound Inc.
// All rights reserved

package processor

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
)

// RedactConfigMaps redacts sensitive data from ConfigMaps.
func RedactConfigMaps(_ context.Context, bundleRoot string) error {
	crDir := clusterResourcesDir(bundleRoot)
	if crDir == "" {
		return nil
	}

	configMapsDir := filepath.Join(crDir, "configmaps")
	if _, err := os.Stat(configMapsDir); os.IsNotExist(err) {
		return nil
	}

	return filepath.Walk(configMapsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}

		data, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			return errors.Wrapf(err, "failed to read file %q", path)
		}

		list := &corev1.ConfigMapList{}
		if err := json.Unmarshal(data, list); err != nil {
			return errors.Wrapf(err, "failed to unmarshal ConfigMap list from %q", path)
		}

		for i := range list.Items {
			list.Items[i].Data = make(map[string]string)
			list.Items[i].BinaryData = make(map[string][]byte)
		}

		jsonData, err := json.MarshalIndent(list, "", "  ")
		if err != nil {
			return errors.Wrapf(err, "failed to marshal redacted data for %q", path)
		}

		if err := os.WriteFile(path, jsonData, 0o600); err != nil {
			return errors.Wrapf(err, "failed to write redacted file %q", path)
		}

		return nil
	})
}

// RedactEnvironmentConfigs redacts sensitive data from EnvironmentConfigs.
func RedactEnvironmentConfigs(_ context.Context, bundleRoot string) error {
	crDir := clusterResourcesDir(bundleRoot)
	if crDir == "" {
		return nil
	}

	customResourcesDir := filepath.Join(crDir, "custom-resources")
	return redactFilesInDir(customResourcesDir, "environmentconfigs.apiextensions.crossplane.io", func(filePath string) error {
		return redactYAMLOrJSONFile(filePath, func(items []unstructured.Unstructured) {
			for i := range items {
				_ = unstructured.SetNestedField(items[i].Object, map[string]any{}, "data")
				annotations := items[i].GetAnnotations()
				if annotations != nil {
					delete(annotations, "kubectl.kubernetes.io/last-applied-configuration")
					items[i].SetAnnotations(annotations)
				}
			}
		})
	})
}

// RedactProviderKubernetesObjects redacts sensitive data from provider-kubernetes Objects (secrets).
func RedactProviderKubernetesObjects(_ context.Context, bundleRoot string) error {
	crDir := clusterResourcesDir(bundleRoot)
	if crDir == "" {
		return nil
	}

	customResourcesDir := filepath.Join(crDir, "custom-resources")
	return redactFilesInDir(customResourcesDir, "objects.kubernetes.crossplane.io", func(filePath string) error {
		return redactYAMLOrJSONFile(filePath, func(items []unstructured.Unstructured) {
			for i := range items {
				redactObjectSecrets(&items[i])
			}
		})
	})
}

func redactObjectSecrets(obj *unstructured.Unstructured) {
	manifest, found, _ := unstructured.NestedMap(obj.Object, "spec", "forProvider", "manifest")
	if !found {
		return
	}

	kind, _, _ := unstructured.NestedString(manifest, "kind")
	if kind != "Secret" {
		return
	}

	// redact data and stringData fields from forProvider manifest
	_ = unstructured.SetNestedField(manifest, map[string]any{}, "data")
	_ = unstructured.SetNestedField(manifest, map[string]any{}, "stringData")
	_ = unstructured.SetNestedField(obj.Object, manifest, "spec", "forProvider", "manifest")

	// redact data and stringData fields from atProvider manifest
	statusManifest, found, _ := unstructured.NestedMap(obj.Object, "status", "atProvider", "manifest")
	if found {
		_ = unstructured.SetNestedField(statusManifest, map[string]any{}, "data")
		_ = unstructured.SetNestedField(statusManifest, map[string]any{}, "stringData")
		manifestMetadata, found, _ := unstructured.NestedMap(statusManifest, "metadata")
		if found {
			annotations, found, _ := unstructured.NestedMap(manifestMetadata, "annotations")
			if found {
				delete(annotations, "kubectl.kubernetes.io/last-applied-configuration")
				_ = unstructured.SetNestedField(manifestMetadata, annotations, "annotations")
				_ = unstructured.SetNestedField(statusManifest, manifestMetadata, "metadata")
			}
		}
		_ = unstructured.SetNestedField(obj.Object, statusManifest, "status", "atProvider", "manifest")
	}

	// Always remove last-applied-configuration annotation from the object
	annotations := obj.GetAnnotations()
	if annotations != nil {
		delete(annotations, "kubectl.kubernetes.io/last-applied-configuration")
		obj.SetAnnotations(annotations)
	}
}

func clusterResourcesDir(bundleRoot string) string {
	dir := filepath.Join(bundleRoot, "cluster-resources")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return ""
	}
	return dir
}

func redactYAMLOrJSONFile(filePath string, redactFunc func([]unstructured.Unstructured)) error {
	data, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		return errors.Wrapf(err, "failed to read file %q", filePath)
	}

	var items []unstructured.Unstructured
	if err := yaml.Unmarshal(data, &items); err != nil {
		return errors.Wrapf(err, "failed to unmarshal supportbundle data from %q", filePath)
	}

	redactFunc(items)

	outputData, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return errors.Wrapf(err, "failed to marshal redacted data for %q", filePath)
	}

	outputPath := strings.TrimSuffix(filePath, filepath.Ext(filePath)) + ".json"
	if filePath != outputPath {
		if err := os.Remove(filePath); err != nil {
			return errors.Wrapf(err, "failed to remove old file %q", filePath)
		}
	}

	if err := os.WriteFile(outputPath, outputData, 0o600); err != nil {
		return errors.Wrapf(err, "failed to write redacted file %q", outputPath)
	}

	return nil
}

func redactFilesInDir(customResourcesDir string, baseName string, redactFunc func(string) error) error {
	if _, err := os.Stat(customResourcesDir); os.IsNotExist(err) {
		return nil
	}

	jsonPath := filepath.Join(customResourcesDir, baseName+".json")
	yamlPath := filepath.Join(customResourcesDir, baseName+".yaml")

	if _, err := os.Stat(jsonPath); err == nil {
		if err := redactFunc(jsonPath); err != nil {
			return err
		}
	}

	if _, err := os.Stat(yamlPath); err == nil {
		if err := redactFunc(yamlPath); err != nil {
			return err
		}
	}

	return nil
}
