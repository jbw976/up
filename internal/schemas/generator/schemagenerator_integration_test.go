// Copyright 2025 Upbound Inc.
// All rights reserved

//go:build integration
// +build integration

package generator

import (
	"context"
	"os"
	"testing"

	"github.com/spf13/afero"
	"gotest.tools/v3/assert"

	"github.com/upbound/up/internal/schemas/runner"

	_ "embed"
)

var (
	//go:embed testdata/account_scaffold_definition.yaml
	testSchemaXrd []byte
	//go:embed testdata/account_scaffold_composition.yaml
	testSchemaComposition []byte
	//go:embed testdata/configuration_crossplane.yaml
	testSchemaConfiguration []byte
)

func TestSchemas(t *testing.T) {
	// Define the function type for file system creation
	type withFsFn func() afero.Fs

	// Define the arguments for the test case
	type args struct {
		fs  withFsFn
		gen Interface
	}

	// Define the expected output (want)
	type want struct {
		err           error
		requiredFiles []string
	}

	// Define the test cases
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"KCL": {
			reason: "Should successfully build KCL schemas.",
			args: args{
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					_ = fs.Mkdir("ws", os.ModePerm)
					_ = fs.Mkdir("ws/apis", os.ModePerm)
					_ = afero.WriteFile(fs, "ws/crossplane.yaml", testSchemaConfiguration, os.ModePerm)
					_ = afero.WriteFile(fs, "ws/apis/composition.yaml", testSchemaComposition, os.ModePerm)
					_ = afero.WriteFile(fs, "ws/apis/definition.yaml", testSchemaXrd, os.ModePerm)
					return fs
				},
				gen: kclGenerator{},
			},
			want: want{
				err: nil,
				requiredFiles: []string{
					"models/co/acme/platform/v1alpha1/accountscaffold.k",
					"models/co/acme/platform/v1alpha1/xaccountscaffold.k",
					"models/k8s/apimachinery/pkg/apis/meta/v1/object_meta.k",
					"models/k8s/apimachinery/pkg/apis/meta/v1/owner_reference.k",
					"models/kcl.mod",
				},
			},
		},
		"Python": {
			reason: "Should successfully build Python schemas.",
			args: args{
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					_ = fs.Mkdir("ws", os.ModePerm)
					_ = fs.Mkdir("ws/apis", os.ModePerm)
					_ = afero.WriteFile(fs, "ws/crossplane.yaml", testSchemaConfiguration, os.ModePerm)
					_ = afero.WriteFile(fs, "ws/apis/composition.yaml", testSchemaComposition, os.ModePerm)
					_ = afero.WriteFile(fs, "ws/apis/definition.yaml", testSchemaXrd, os.ModePerm)
					return fs
				},
				gen: pythonGenerator{},
			},
			want: want{
				err: nil,
				requiredFiles: []string{
					"models/co/acme/platform/accountscaffold/__init__.py",
					"models/co/acme/platform/accountscaffold/v1alpha1.py",
					"models/co/acme/platform/xaccountscaffold/__init__.py",
					"models/co/acme/platform/xaccountscaffold/v1alpha1.py",
					"models/io/k8s/apimachinery/pkg/apis/meta/__init__.py",
					"models/io/k8s/apimachinery/pkg/apis/meta/v1.py",
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Initialize the in-memory file system from the test case
			fromFS := tc.args.fs()

			schemaFS, err := tc.args.gen.Generate(context.Background(), fromFS, runner.NewRealSchemaRunner())
			assert.NilError(t, err)

			for _, file := range tc.want.requiredFiles {
				_, err := schemaFS.Open(file)
				assert.NilError(t, err, "expected file %s but it was not found", file)
			}
		})
	}
}
