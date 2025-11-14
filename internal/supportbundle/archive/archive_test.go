// Copyright 2025 Upbound Inc.
// All rights reserved

package archive

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/upbound/up/internal/supportbundle/testutil"
)

func TestExtract(t *testing.T) {
	tests := []struct {
		name      string
		setupTar  func() string
		wantFiles []string
		wantErr   error
	}{
		{
			name: "ExtractSimpleArchive",
			setupTar: func() string {
				return testutil.CreateTestTar(t, map[string]string{
					"file1.txt": "content1",
					"file2.txt": "content2",
				})
			},
			wantFiles: []string{"file1.txt", "file2.txt"},
			wantErr:   nil,
		},
		{
			name: "ExtractWithDirectories",
			setupTar: func() string {
				return testutil.CreateTestTar(t, map[string]string{
					"dir1/file1.txt": "content1",
					"dir2/file2.txt": "content2",
				})
			},
			wantFiles: []string{"dir1/file1.txt", "dir2/file2.txt"},
			wantErr:   nil,
		},
		{
			name: "ExtractEmptyArchive",
			setupTar: func() string {
				return testutil.CreateTestTar(t, map[string]string{})
			},
			wantFiles: []string{},
			wantErr:   nil,
		},
		{
			name: "ExtractNonExistentArchive",
			setupTar: func() string {
				return "/nonexistent/archive.tar.gz"
			},
			wantFiles: []string{},
			wantErr:   cmpopts.AnyError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			archivePath := tt.setupTar()
			destDir := t.TempDir()

			err := Extract(archivePath, destDir)

			if diff := cmp.Diff(tt.wantErr, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nExtract(...): -want err, +got err\n%s", tt.name, diff)
			}

			for _, file := range tt.wantFiles {
				filePath := filepath.Join(destDir, file)
				if _, err := os.Stat(filePath); err != nil {
					t.Errorf("%s\nexpected file %s to exist: %v", tt.name, file, err)
				}
			}
		})
	}
}

func TestFindBundleRoot(t *testing.T) {
	tests := []struct {
		name      string
		setupDir  func() string
		wantFound bool
		wantErr   error
	}{
		{
			name: "FindSupportBundleDirectory",
			setupDir: func() string {
				dir := t.TempDir()
				_ = os.MkdirAll(filepath.Join(dir, "support-bundle-20250105-163905"), 0o755)
				_ = os.MkdirAll(filepath.Join(dir, "other-dir"), 0o755)
				return dir
			},
			wantFound: true,
			wantErr:   nil,
		},
		{
			name: "NoSupportBundleDirectory",
			setupDir: func() string {
				dir := t.TempDir()
				_ = os.MkdirAll(filepath.Join(dir, "other-dir"), 0o755)
				return dir
			},
			wantFound: false,
			wantErr:   nil,
		},
		{
			name: "EmptyDirectory",
			setupDir: func() string {
				return t.TempDir()
			},
			wantFound: false,
			wantErr:   nil,
		},
		{
			name: "CustomBundleName",
			setupDir: func() string {
				dir := t.TempDir()
				customBundleDir := filepath.Join(dir, "my-custom-bundle-name")
				_ = os.MkdirAll(filepath.Join(customBundleDir, "cluster-resources"), 0o755)
				_ = os.MkdirAll(filepath.Join(dir, "other-dir"), 0o755)
				return dir
			},
			wantFound: true,
			wantErr:   nil,
		},
		{
			name: "BundleRootIsTempDir",
			setupDir: func() string {
				dir := t.TempDir()
				_ = os.MkdirAll(filepath.Join(dir, "cluster-resources"), 0o755)
				return dir
			},
			wantFound: false,
			wantErr:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := tt.setupDir()

			bundleRoot, err := FindBundleRoot(tempDir)

			if diff := cmp.Diff(tt.wantErr, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nFindBundleRoot(...): -want err, +got err\n%s", tt.name, diff)
			}
			if err != nil {
				return
			}

			if tt.wantFound {
				if bundleRoot == tempDir {
					t.Errorf("%s\nshould find support-bundle directory, not return tempDir", tt.name)
				}
				if filepath.Base(bundleRoot) == "." {
					t.Errorf("%s\nbundle root should not be '.'", tt.name)
				}
			} else {
				if diff := cmp.Diff(tempDir, bundleRoot); diff != "" {
					t.Errorf("%s\nshould return tempDir when no support-bundle directory found (-want +got):\n%s", tt.name, diff)
				}
			}
		})
	}
}
