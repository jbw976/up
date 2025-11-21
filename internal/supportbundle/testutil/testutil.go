// Copyright 2025 Upbound Inc.
// All rights reserved

// Package testutil provides test utilities for support bundle operations.
package testutil

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

// CreateTestTar creates a temporary tar.gz archive with the given files.
// The files map keys are file paths, and values are file contents.
// Returns the path to the created archive file.
func CreateTestTar(t *testing.T, files map[string]string) string {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "test-archive-*.tar.gz")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	archivePath := tmpFile.Name()
	t.Cleanup(func() {
		if err := os.Remove(archivePath); err != nil {
			t.Logf("failed to remove temp file: %v", err)
		}
	})
	defer func() {
		if closeErr := tmpFile.Close(); closeErr != nil {
			t.Logf("failed to close temp file: %v", closeErr)
		}
	}()

	gzw := gzip.NewWriter(tmpFile)
	defer func() {
		if closeErr := gzw.Close(); closeErr != nil {
			t.Logf("failed to close gzip writer: %v", closeErr)
		}
	}()

	tw := tar.NewWriter(gzw)
	defer func() {
		if closeErr := tw.Close(); closeErr != nil {
			t.Logf("failed to close tar writer: %v", closeErr)
		}
	}()

	for path, content := range files {
		dir := filepath.Dir(path)
		if dir != "." {
			hdr := &tar.Header{
				Name:     dir + "/",
				Typeflag: tar.TypeDir,
				Mode:     0o755,
			}
			if err := tw.WriteHeader(hdr); err != nil {
				t.Fatalf("failed to write directory header: %v", err)
			}
		}

		hdr := &tar.Header{
			Name:     path,
			Size:     int64(len(content)),
			Mode:     0o644,
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("failed to write file header: %v", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write file content: %v", err)
		}
	}

	return archivePath
}
