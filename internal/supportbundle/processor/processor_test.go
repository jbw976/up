// Copyright 2025 Upbound Inc.
// All rights reserved

package processor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/upbound/up/internal/supportbundle/archive"
	"github.com/upbound/up/internal/supportbundle/testutil"
)

func TestApply(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T) string
		processors []Func
		wantErr    error
		verify     func(archivePath string, t *testing.T)
	}{
		{
			name: "NoProcessors",
			setup: func(t *testing.T) string {
				return testutil.CreateTestTar(t, map[string]string{
					"file1.txt": "content1",
				})
			},
			processors: []Func{},
			wantErr:    nil,
			verify:     func(_ string, _ *testing.T) {},
		},
		{
			name: "WithProcessor",
			setup: func(t *testing.T) string {
				return testutil.CreateTestTar(t, map[string]string{
					"cluster-resources/configmaps/cm.json": `{"kind":"ConfigMap","data":{"key":"value"}}`,
				})
			},
			processors: []Func{
				func(_ context.Context, bundleRoot string) error {
					markerPath := filepath.Join(bundleRoot, "processed.txt")
					return os.WriteFile(markerPath, []byte("processed"), 0o600)
				},
			},
			wantErr: nil,
			verify: func(archivePath string, t *testing.T) {
				tempDir := t.TempDir()
				if err := archive.Extract(archivePath, tempDir); err != nil {
					t.Fatalf("failed to extract archive: %v", err)
				}

				bundleRoot, err := archive.FindBundleRoot(tempDir)
				if err != nil {
					t.Fatalf("failed to find bundle root: %v", err)
				}

				markerPath := filepath.Join(bundleRoot, "processed.txt")
				if _, err := os.Stat(markerPath); err != nil {
					t.Errorf("marker file should exist: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			archivePath := tt.setup(t)

			err := Apply(context.Background(), archivePath, tt.processors...)

			if diff := cmp.Diff(tt.wantErr, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nApply(...): -want err, +got err\n%s", tt.name, diff)
			}
			if err != nil {
				return
			}
			tt.verify(archivePath, t)
		})
	}
}
