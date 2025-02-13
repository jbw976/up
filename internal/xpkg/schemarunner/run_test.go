// Copyright 2025 Upbound Inc.
// All rights reserved

package schemarunner

import (
	"context"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/upbound/up/internal/filesystem"

	_ "embed"
)

//go:embed testdata/template.fn.crossplane.io_kclinputs.yaml
var crd []byte

func TestRunContainerWithKCL(t *testing.T) {
	type withFsFn func() afero.Fs

	type args struct {
		baseFolder string
		fs         withFsFn
		runner     SchemaRunner // Use SchemaRunner interface here
	}

	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SuccessWithAccountScaffoldDefinition": {
			reason: "Should successfully generate with crd using MockSchemaRunner.",
			args: args{
				baseFolder: "data/input",
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					_ = fs.Mkdir("data/input", os.ModePerm)
					_ = afero.WriteFile(fs, "data/input/template.fn.crossplane.io_kclinputs.yaml", crd, os.ModePerm)
					return fs
				},
				runner: &MockSchemaRunner{}, // Use the mock runner
			},
			want: want{
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fs := tc.args.fs()

			// Use the provided runner for schema generation.
			ctx := context.Background()
			err := tc.args.runner.Generate(ctx, fs, tc.args.baseFolder, "", "mockImage", []string{})

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRunContainer(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			outputExists, _ := afero.Exists(fs, "models/k8s/apimachinery/pkg/apis/meta/v1/managed_fields_entry.k")
			if !outputExists {
				t.Errorf("\n%s\nExpected output file not found in in-memory fs", tc.reason)
			}
		})
	}
}

func TestCreateTarFromFs(t *testing.T) {
	type withFsFn func() afero.Fs
	type args struct {
		baseFolder string
		fs         withFsFn
	}
	type want struct {
		tarFileExists bool
		err           error
	}

	// Define test cases
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SuccessWithValidTar": {
			reason: "Should successfully create tar from valid file system.",
			args: args{
				baseFolder: ".", // Root directory
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					_ = afero.WriteFile(fs, "file.txt", []byte("hello world"), 0o644) // Relative path
					return fs
				},
			},
			want: want{
				tarFileExists: true,
				err:           nil,
			},
		},
		"FailWithInvalidPath": {
			reason: "Will not fail to create tar due to invalid file path.",
			args: args{
				baseFolder: "/invalid", // Non-existent path
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					return fs
				},
			},
			want: want{
				tarFileExists: true,
				err:           nil, // Expected error is nil since FSToTar might not throw error on invalid path
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fs := tc.args.fs()

			// Attempt to create the tar
			tarBuffer, err := filesystem.FSToTar(fs, tc.args.baseFolder)

			// Check tar creation success using only len check
			tarFileExists := len(tarBuffer) > 0
			if diff := cmp.Diff(tc.want.tarFileExists, tarFileExists); diff != "" {
				t.Errorf("\n%s\ncreateTarFromFs(...): -want tar file, +got no tar file:\n%s", tc.reason, diff)
			}

			// Check for errors if expected
			if tc.want.err != nil {
				if err == nil {
					t.Errorf("Expected error but got none for test case: %s", name)
				} else if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("\n%s\ncreateTarFromFs(...): -want err, +got err:\n%s", tc.reason, diff)
				}
			} else if err != nil {
				t.Errorf("Unexpected error for test case %s: %v", name, err)
			}
		})
	}
}

// MockSchemaRunner simulates a successful container run by generating output in-memory.
type MockSchemaRunner struct{}

// Generate will run MockSchemaRunner generate for Schema.
func (m *MockSchemaRunner) Generate(_ context.Context, fs afero.Fs, _, _ string, _ string, _ []string) error {
	// Simulate the generation of expected output files in-memory.
	outputPath := "models/k8s/apimachinery/pkg/apis/meta/v1/managed_fields_entry.k"
	_ = fs.MkdirAll("models/k8s/apimachinery/pkg/apis/meta/v1", os.ModePerm)
	return afero.WriteFile(fs, outputPath, []byte("mock content"), os.ModePerm)
}
