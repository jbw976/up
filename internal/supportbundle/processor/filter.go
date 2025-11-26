// Copyright 2025 Upbound Inc.
// All rights reserved

package processor

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
)

var (
	// crossplaneCategories lists the CRD categories that identify Crossplane resources.
	//nolint:gochecknoglobals // These are configuration constants used across the package.
	crossplaneCategories = []string{"composites", "crossplane", "managed"}
	// crossplaneAPIGroups lists the API group suffixes that identify Crossplane resources.
	//nolint:gochecknoglobals // These are configuration constants used across the package.
	crossplaneAPIGroups = []string{".crossplane.io", ".upbound.io"}
)

// FilterCrossplaneResources creates a processor that filters to only Crossplane resources.
func FilterCrossplaneResources(_ context.Context, bundleRoot string) error {
	clusterResourcesDir := filepath.Join(bundleRoot, "cluster-resources")

	if _, err := os.Stat(clusterResourcesDir); os.IsNotExist(err) {
		return nil
	}

	crdFilePath := filepath.Join(clusterResourcesDir, "custom-resource-definitions.json")
	keptCRDNames, err := filterCRDList(crdFilePath)
	if err != nil {
		return errors.Wrap(err, "failed to filter CRDs")
	}

	customResourcesDir := filepath.Join(clusterResourcesDir, "custom-resources")
	if err := filterCustomResources(customResourcesDir, keptCRDNames); err != nil {
		return errors.Wrap(err, "failed to filter custom resources")
	}

	if err := removeNonCrossplaneClusterResources(clusterResourcesDir); err != nil {
		return errors.Wrap(err, "failed to remove non-Crossplane cluster resources")
	}

	if err := removeNonEssentialBundleDirectories(bundleRoot); err != nil {
		return errors.Wrap(err, "failed to remove non-essential bundle directories")
	}

	return nil
}

func isCrossplaneCRD(crd apiextensionsv1.CustomResourceDefinition) bool {
	for _, groupSuffix := range crossplaneAPIGroups {
		if strings.HasSuffix(crd.Spec.Group, groupSuffix) {
			return true
		}
	}

	return slices.ContainsFunc(crd.Spec.Names.Categories, func(cat string) bool {
		return slices.Contains(crossplaneCategories, cat)
	})
}

func filterCRDList(filePath string) (map[string]bool, error) {
	keptCRDNames := make(map[string]bool)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return keptCRDNames, nil
	}

	data, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read CRD file %q", filePath)
	}

	var list apiextensionsv1.CustomResourceDefinitionList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal CRD list from %q", filePath)
	}

	var keptCRDs []apiextensionsv1.CustomResourceDefinition
	for _, crd := range list.Items {
		if isCrossplaneCRD(crd) {
			keptCRDs = append(keptCRDs, crd)
			keptCRDNames[crd.Name] = true
			if crd.Spec.Names.Plural != "" && crd.Spec.Group != "" {
				resourceTypeName := crd.Spec.Names.Plural + "." + crd.Spec.Group
				keptCRDNames[resourceTypeName] = true
			}
		}
	}

	list.Items = keptCRDs
	filteredData, err := json.Marshal(list)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to marshal filtered CRDs")
	}

	if err := os.WriteFile(filePath, filteredData, 0o600); err != nil {
		return nil, errors.Wrapf(err, "failed to write filtered CRD file %q", filePath)
	}

	return keptCRDNames, nil
}

func filterCustomResources(customResourcesDir string, keptCRDNames map[string]bool) error {
	if _, err := os.Stat(customResourcesDir); os.IsNotExist(err) {
		return nil
	}

	entries, err := os.ReadDir(customResourcesDir)
	if err != nil {
		return errors.Wrapf(err, "failed to read custom-resources directory %q", customResourcesDir)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fileName := entry.Name()
		if strings.Contains(fileName, "custom-resource") && strings.Contains(fileName, "error") {
			continue
		}

		resourceTypeName := strings.TrimSuffix(fileName, filepath.Ext(fileName))

		if !keptCRDNames[resourceTypeName] {
			filePath := filepath.Join(customResourcesDir, fileName)
			if err := os.Remove(filePath); err != nil {
				return errors.Wrapf(err, "failed to remove non-Crossplane resource file %q", filePath)
			}
		}
	}

	return nil
}

func removeNonCrossplaneClusterResources(clusterResourcesDir string) error {
	entries, err := os.ReadDir(clusterResourcesDir)
	if err != nil {
		return errors.Wrapf(err, "failed to read cluster-resources directory %q", clusterResourcesDir)
	}

	keepItems := map[string]bool{
		"custom-resource-definitions.json": true,
		"custom-resources":                 true,
		"namespaces.json":                  true,
		"resources.json":                   true,
	}

	for _, entry := range entries {
		if keepItems[entry.Name()] {
			continue
		}

		itemPath := filepath.Join(clusterResourcesDir, entry.Name())
		if entry.IsDir() {
			if err := os.RemoveAll(itemPath); err != nil {
				return errors.Wrapf(err, "failed to remove directory %q", itemPath)
			}
		} else {
			if err := os.Remove(itemPath); err != nil {
				return errors.Wrapf(err, "failed to remove file %q", itemPath)
			}
		}
	}

	return nil
}

func removeNonEssentialBundleDirectories(bundleRoot string) error {
	entries, err := os.ReadDir(bundleRoot)
	if err != nil {
		return errors.Wrapf(err, "failed to read bundle root %q", bundleRoot)
	}

	keepDirs := map[string]bool{
		"cluster-resources": true,
		"execution-data":    true,
		"cluster-info":      true,
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		if keepDirs[entry.Name()] {
			continue
		}

		itemPath := filepath.Join(bundleRoot, entry.Name())
		if err := os.RemoveAll(itemPath); err != nil {
			return errors.Wrapf(err, "failed to remove directory %q", itemPath)
		}
	}

	return nil
}
