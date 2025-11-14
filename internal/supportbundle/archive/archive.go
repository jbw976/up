// Copyright 2025 Upbound Inc.
// All rights reserved

// Package archive provides utilities for extracting and repackaging support bundle archives.
package archive

import (
	"os"
	"path/filepath"
	"strings"

	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"github.com/replicatedhq/troubleshoot/pkg/collect"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
)

// Extract extracts a tar.gz archive to a directory using troubleshoot's ExtractTroubleshootBundle function.
func Extract(archivePath, destDir string) error {
	file, err := os.Open(filepath.Clean(archivePath))
	if err != nil {
		return errors.Wrapf(err, "failed to open archive %q", archivePath)
	}
	defer file.Close() //nolint:errcheck // extracting could have succeeded

	if err := analyzer.ExtractTroubleshootBundle(file, destDir); err != nil {
		return errors.Wrap(err, "failed to extract archive")
	}

	return nil
}

// Repackage creates a new tar.gz archive from a directory using troubleshoot's ArchiveBundle function.
func Repackage(sourceDir, archivePath string) error {
	if err := os.Remove(archivePath); err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "failed to remove existing archive %q", archivePath)
	}

	result, err := collect.CollectorResultFromBundle(sourceDir)
	if err != nil {
		return errors.Wrap(err, "failed to create collector result from bundle directory")
	}

	if err := result.ArchiveBundle(sourceDir, archivePath); err != nil {
		return errors.Wrap(err, "failed to archive bundle")
	}

	return nil
}

// FindBundleRoot finds the bundle root directory within a temp directory.
func FindBundleRoot(tempDir string) (string, error) {
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return "", errors.Wrap(err, "failed to read temp directory")
	}

	var candidates []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirPath := filepath.Join(tempDir, entry.Name())

		if strings.HasPrefix(entry.Name(), "support-bundle-") {
			return dirPath, nil
		}

		clusterResourcesPath := filepath.Join(dirPath, "cluster-resources")
		if info, err := os.Stat(clusterResourcesPath); err == nil && info.IsDir() {
			candidates = append(candidates, dirPath)
		}
	}

	if len(candidates) == 1 {
		return candidates[0], nil
	}

	clusterResourcesPath := filepath.Join(tempDir, "cluster-resources")
	if info, err := os.Stat(clusterResourcesPath); err == nil && info.IsDir() {
		return tempDir, nil
	}

	return tempDir, nil
}
