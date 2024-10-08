// Copyright 2024 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package schemagenerator

import (
	_ "embed"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/afero"
)

// TestReorganizeAndAdjustImports tests reorganizing files and adjusting imports.
func TestReorganizeAndAdjustImports(t *testing.T) {

	// Test case structure
	tests := []struct {
		name           string
		setupFs        func(fs afero.Fs) // Setup for the filesystem
		sourceDir      string
		targetDir      string
		expectedFiles  map[string]string // expected file paths and their content
		expectedErrors bool
	}{
		{
			name: "BasicReorganizationAndImportAdjustment",
			setupFs: func(fs afero.Fs) {
				afero.WriteFile(fs, pythonGeneratedFolder+"/platform_acme_co_v1alpha1_subnetwork/io/k8s/apimachinery/pkg/apis/meta/v1.py", []byte("from __future__ import annotations"), os.ModePerm)
				afero.WriteFile(fs, pythonGeneratedFolder+"/platform_acme_co_v1alpha1_subnetwork/io/k8s/apimachinery/pkg/apis/meta/__init__.py", []byte(""), os.ModePerm)
				afero.WriteFile(fs, pythonGeneratedFolder+"/platform_acme_co_v1alpha1_subnetwork/co/acme/platform/v1alpha1.py", []byte("from ....io.k8s.apimachinery.pkg.apis.meta import v1"), os.ModePerm)
				afero.WriteFile(fs, pythonGeneratedFolder+"/platform_acme_co_v1alpha1_subnetwork/co/acme/platform/__init__.py", []byte(""), os.ModePerm)
				afero.WriteFile(fs, pythonGeneratedFolder+"/platform_acme_co_v1alpha1_compositecluster/io/k8s/apimachinery/pkg/apis/meta/v1.py", []byte("from __future__ import annotations"), os.ModePerm)
				afero.WriteFile(fs, pythonGeneratedFolder+"/platform_acme_co_v1alpha1_compositecluster/io/k8s/apimachinery/pkg/apis/meta/__init__.py", []byte(""), os.ModePerm)
				afero.WriteFile(fs, pythonGeneratedFolder+"/platform_acme_co_v1alpha1_compositecluster/co/acme/platform/v1alpha1.py", []byte("from ....io.k8s.apimachinery.pkg.apis.meta import v1"), os.ModePerm)
				afero.WriteFile(fs, pythonGeneratedFolder+"/platform_acme_co_v1alpha1_compositecluster/co/acme/platform/__init__.py", []byte(""), os.ModePerm)
			},
			sourceDir: pythonGeneratedFolder,
			targetDir: pythonAdoptModelsStructure,
			expectedFiles: map[string]string{
				pythonAdoptModelsStructure + "/io/k8s/apimachinery/pkg/apis/meta/v1.py":       "from __future__ import annotations",
				pythonAdoptModelsStructure + "/io/k8s/apimachinery/pkg/apis/meta/__init__.py": "",
				pythonAdoptModelsStructure + "/co/acme/platform/subnetwork/v1alpha1.py":       "from .....io.k8s.apimachinery.pkg.apis.meta import v1",
				pythonAdoptModelsStructure + "/co/acme/platform/subnetwork/__init__.py":       "",
				pythonAdoptModelsStructure + "/co/acme/platform/compositecluster/v1alpha1.py": "from .....io.k8s.apimachinery.pkg.apis.meta import v1",
				pythonAdoptModelsStructure + "/co/acme/platform/compositecluster/__init__.py": "",
			},
			expectedErrors: false,
		},
	}

	// Iterate over test cases
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			tt.setupFs(fs)
			err := reorganizeAndAdjustImports(fs, tt.sourceDir, tt.targetDir)

			if (err != nil) != tt.expectedErrors {
				t.Fatalf("Expected error: %v, got: %v", tt.expectedErrors, err)
			}

			// Validate the resulting file structure and content
			for expectedFile, expectedContent := range tt.expectedFiles {
				data, err := afero.ReadFile(fs, expectedFile)
				if err != nil {
					t.Fatalf("Expected file %s does not exist: %v", expectedFile, err)
				}
				content := string(data)
				if diff := cmp.Diff(expectedContent, content); diff != "" {
					t.Errorf("File %s content mismatch (-want +got):\n%s", expectedFile, diff)
				}
			}
		})
	}
}

func TestAdjustLeadingDots(t *testing.T) {
	tests := []struct {
		name       string
		importLine string
		depth      int
		expected   string
	}{
		{
			name:       "NoAdjustmentNeeded",
			importLine: "from io.k8s.apimachinery.pkg.apis.meta import v1",
			depth:      0,
			expected:   "from io.k8s.apimachinery.pkg.apis.meta import v1",
		},
		{
			name:       "OneLevelDeep",
			importLine: "from ..io.k8s.apimachinery.pkg.apis.meta import v1",
			depth:      1,
			expected:   "from .io.k8s.apimachinery.pkg.apis.meta import v1",
		},
		{
			name:       "ThreeLevelsDeep",
			importLine: "from io.k8s.apimachinery.pkg.apis.meta import v1",
			depth:      3,
			expected:   "from ...io.k8s.apimachinery.pkg.apis.meta import v1",
		},
		{
			name:       "AlreadyContainsLeadingDots",
			importLine: "from ......io.k8s.apimachinery.pkg.apis.meta import v1",
			depth:      2,
			expected:   "from ..io.k8s.apimachinery.pkg.apis.meta import v1",
		},
		{
			name:       "NonMatchingImport",
			importLine: "from some.other.module import something",
			depth:      3,
			expected:   "from some.other.module import something",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := adjustLeadingDots(tt.importLine, tt.depth)
			if got != tt.expected {
				t.Errorf("adjustLeadingDots() = %v, want %v", got, tt.expected)
			}
		})
	}
}
