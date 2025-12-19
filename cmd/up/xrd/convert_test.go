// Copyright 2025 Upbound Inc.
// All rights reserved

package xrd

import (
	"embed"
	"testing"

	"github.com/spf13/afero"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"

	"github.com/upbound/up/internal/upterm"
)

//go:embed testdata/*
var testdataFS embed.FS

func TestConvertCmd_Run(t *testing.T) {
	type want struct {
		err           bool
		expectedFiles []string
	}

	cases := map[string]struct {
		xrdFile string
		want    want
	}{
		"V1XRWithoutClaim": {
			xrdFile: "xr-definition.yaml",
			want: want{
				err: false,
				expectedFiles: []string{
					"xnetworks.aws.platform.upbound.io.yaml",
				},
			},
		},
		"V1XRWithClaim": {
			xrdFile: "claim-definition.yaml",
			want: want{
				err: false,
				expectedFiles: []string{
					"xclusters.aws.platformref.upbound.io.yaml",
					"clusters.aws.platformref.upbound.io.yaml",
				},
			},
		},
		"V2ClusterScoped": {
			xrdFile: "v2-definition-cluster.yaml",
			want: want{
				err: false,
				expectedFiles: []string{
					"webapps.platform.example.com.yaml",
				},
			},
		},
		"V2NamespaceScoped": {
			xrdFile: "v2-definition-ns.yaml",
			want: want{
				err: false,
				expectedFiles: []string{
					"webapps.platform.example.com.yaml",
				},
			},
		},
		"InvalidXRD": {
			xrdFile: "",
			want: want{
				err:           true,
				expectedFiles: []string{},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Create base filesystem from embedded testdata
			testdataBaseFS := afero.NewBasePathFs(afero.FromIOFS{FS: testdataFS}, "testdata")

			// Create in-memory filesystem for output
			fs := afero.NewMemMapFs()

			var xrdContent []byte
			var err error

			// Read XRD file from embedded FS if provided
			if tc.xrdFile != "" {
				xrdContent, err = afero.ReadFile(testdataBaseFS, tc.xrdFile)
				if err != nil {
					t.Fatalf("failed to read embedded XRD file: %v", err)
				}
			} else {
				// For invalid test case
				xrdContent = []byte("invalid yaml content [[[")
			}

			// Write the XRD file to in-memory FS
			xrdPath := "/input/xrd.yaml"
			err = afero.WriteFile(fs, xrdPath, xrdContent, 0o644)
			if err != nil {
				t.Fatalf("failed to write XRD file: %v", err)
			}

			// Create the command
			outputDir := "/output"
			cmd := &convertCmd{
				File:      xrdPath,
				OutputDir: outputDir,
				fs:        fs,
			}

			// Run the command
			err = cmd.Run(upterm.NewTestPrinter())

			// Check error expectation
			if tc.want.err && err == nil {
				t.Error("expected error but got none")
			}
			if !tc.want.err && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// If we expect an error, don't check file contents
			if tc.want.err {
				return
			}

			// Check that all expected files exist and are valid CRDs
			for _, expectedFile := range tc.want.expectedFiles {
				filePath := outputDir + "/" + expectedFile
				exists, err := afero.Exists(fs, filePath)
				if err != nil {
					t.Errorf("failed to check file existence for %s: %v", expectedFile, err)
				}
				if !exists {
					t.Errorf("expected file %s to exist but it doesn't", expectedFile)
					continue
				}

				// Validate it's a valid CRD
				data, err := afero.ReadFile(fs, filePath)
				if err != nil {
					t.Errorf("failed to read file %s: %v", expectedFile, err)
					continue
				}
				var crd apiextensionsv1.CustomResourceDefinition
				if err := yaml.Unmarshal(data, &crd); err != nil {
					t.Errorf("file %s is not valid YAML: %v", expectedFile, err)
					continue
				}
				if crd.Kind != "CustomResourceDefinition" {
					t.Errorf("expected Kind to be CustomResourceDefinition in %s, got %s", expectedFile, crd.Kind)
				}
			}
		})
	}
}
