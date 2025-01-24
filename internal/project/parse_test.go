// Copyright 2025 Upbound Inc.
// All rights reserved

package project

import (
	"os"
	"testing"

	"github.com/spf13/afero"
	"gotest.tools/v3/assert"

	"github.com/upbound/up/pkg/apis/project/v1alpha1"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name          string
		setupFs       func(fs afero.Fs)
		projectFile   string
		expectErr     bool
		expectedPaths *v1alpha1.ProjectPaths
	}{
		{
			name: "ValidProjectFileAllPaths",
			setupFs: func(fs afero.Fs) {
				yamlContent := `
apiVersion: v1alpha1
kind: Project
metadata:
  name: ValidProjectFileAllPaths
spec:
  repository: xpkg.upbound.io/upbound/getting-started
  paths:
    apis: "test"
    examples: "example"
    functions: "funcs"
`
				afero.WriteFile(fs, "/project.yaml", []byte(yamlContent), os.ModePerm)
			},
			projectFile: "/project.yaml",
			expectErr:   false,
			expectedPaths: &v1alpha1.ProjectPaths{
				APIs:      "test",
				Examples:  "example",
				Functions: "funcs",
			},
		},
		{
			name: "ValidProjectFileSomePaths",
			setupFs: func(fs afero.Fs) {
				yamlContent := `
apiVersion: v1alpha1
kind: Project
metadata:
  name: ValidProjectFileSomePaths
spec:
  repository: xpkg.upbound.io/upbound/getting-started
  paths:
    functions: "funcs"
`
				afero.WriteFile(fs, "/project.yaml", []byte(yamlContent), os.ModePerm)
			},
			projectFile: "/project.yaml",
			expectErr:   false,
			expectedPaths: &v1alpha1.ProjectPaths{
				Functions: "funcs",
			},
		},
		{
			name: "InvalidProjectFileYAML",
			setupFs: func(fs afero.Fs) {
				afero.WriteFile(fs, "/project.yaml", []byte("invalid yaml content"), os.ModePerm)
			},
			projectFile:   "/project.yaml",
			expectErr:     true,
			expectedPaths: nil,
		},
		{
			name: "ProjectFileWithNoPaths",
			setupFs: func(fs afero.Fs) {
				yamlContent := `
apiVersion: v1alpha1
kind: Project
metadata:
  name: ProjectFileWithNoPaths
spec:
  repository: xpkg.upbound.io/upbound/getting-started
`
				afero.WriteFile(fs, "/project.yaml", []byte(yamlContent), os.ModePerm)
			},
			projectFile: "/project.yaml",
			expectErr:   false,
		},
		{
			name: "ProjectFileNotFound",
			setupFs: func(_ afero.Fs) {
			},
			projectFile:   "/nonexistent.yaml",
			expectErr:     true,
			expectedPaths: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := afero.NewMemMapFs()

			tt.setupFs(fs)

			proj, err := Parse(fs, tt.projectFile)

			if tt.expectErr {
				assert.Assert(t, err != nil)
				return
			}

			assert.NilError(t, err)
			assert.DeepEqual(t, tt.expectedPaths, proj.Spec.Paths)
		})
	}
}
