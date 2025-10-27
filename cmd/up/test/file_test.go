// Copyright 2025 Upbound Inc.
// All rights reserved

package test

import (
	"fmt"
	"testing"

	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestWriteToFile(t *testing.T) {
	tests := []struct {
		name           string
		resources      []runtime.RawExtension
		filename       string
		expectErr      bool
		expectedOutput string
	}{
		{
			name:     "ValidSingleResource",
			filename: "test-file",
			resources: []runtime.RawExtension{
				{
					Raw: []byte(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key: value
`),
				},
			},
			expectErr: false,
			expectedOutput: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key: value
---
`,
		},
		{
			name:     "MultipleResources",
			filename: "multi-resource",
			resources: []runtime.RawExtension{
				{
					Raw: []byte(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: config1
data:
  key: value1
`),
				},
				{
					Raw: []byte(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: config2
data:
  key: value2
`),
				},
			},
			expectErr: false,
			expectedOutput: `apiVersion: v1
kind: ConfigMap
metadata:
  name: config1
data:
  key: value1
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: config2
data:
  key: value2
---
`,
		},
		{
			name:           "NoResources",
			filename:       "empty-file",
			resources:      []runtime.RawExtension{},
			expectErr:      false,
			expectedOutput: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup in-memory filesystem
			fs := afero.NewMemMapFs()

			// Call function
			filePath, err := writeToFile(fs, tt.resources, tt.filename)

			// Check error expectation
			if tt.expectErr {
				if err == nil {
					t.Fatal("expected an error but got nil")
				}

				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// If there were no resources, filePath should be empty
			if len(tt.resources) == 0 {
				if filePath != "" {
					t.Errorf("expected filePath to be empty, but got: %s", filePath)
				}
				return
			}

			// Verify file path
			expectedPath := fmt.Sprintf("/resources/%s.yaml", tt.filename)
			if expectedPath != filePath {
				t.Errorf("Expected %v, but got %v", expectedPath, filePath)
			}

			// Read back the file content
			actualContent, err := afero.ReadFile(fs, filePath)
			if err != nil {
				t.Fatal(err)
			}

			// Compare expected and actual file content
			if tt.expectedOutput != string(actualContent) {
				t.Errorf("Expected %v, but got %v", tt.expectedOutput, string(actualContent))
			}
		})
	}
}

func TestWriteContextToFile(t *testing.T) {
	tests := []struct {
		name             string
		value            runtime.RawExtension
		filename         string
		expectErr        bool
		expectedOutput   string
		expectedFilePath string
	}{
		{
			name:     "ValidContextValue",
			filename: "context-key1",
			value: runtime.RawExtension{
				Raw: []byte(`{"data": "value", "nested": {"field": 123}}`),
			},
			expectErr:        false,
			expectedOutput:   `{"data": "value", "nested": {"field": 123}}`,
			expectedFilePath: "/resources/context-key1.json",
		},
		{
			name:     "SimpleContextValue",
			filename: "context-simple",
			value: runtime.RawExtension{
				Raw: []byte(`{"key": "value"}`),
			},
			expectErr:        false,
			expectedOutput:   `{"key": "value"}`,
			expectedFilePath: "/resources/context-simple.json",
		},
		{
			name:             "EmptyContextValue",
			filename:         "context-empty",
			value:            runtime.RawExtension{Raw: []byte{}},
			expectErr:        false,
			expectedOutput:   "",
			expectedFilePath: "",
		},
		{
			name:     "ContextWithArray",
			filename: "context-array",
			value: runtime.RawExtension{
				Raw: []byte(`{"items": [1, 2, 3], "enabled": true}`),
			},
			expectErr:        false,
			expectedOutput:   `{"items": [1, 2, 3], "enabled": true}`,
			expectedFilePath: "/resources/context-array.json",
		},
		{
			name:     "FilenameWithSlash",
			filename: "context-apiextensions.crossplane.io/environment",
			value: runtime.RawExtension{
				Raw: []byte(`{"account_id": "123456789"}`),
			},
			expectErr:        false,
			expectedOutput:   `{"account_id": "123456789"}`,
			expectedFilePath: "/resources/context-apiextensions.crossplane.io-environment.json",
		},
		{
			name:     "FilenameWithMultipleSlashes",
			filename: "context-acme.com/prod/us-east-1",
			value: runtime.RawExtension{
				Raw: []byte(`{"region": "us-east-1"}`),
			},
			expectErr:        false,
			expectedOutput:   `{"region": "us-east-1"}`,
			expectedFilePath: "/resources/context-acme.com-prod-us-east-1.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup in-memory filesystem
			fs := afero.NewMemMapFs()

			// Call function
			filePath, err := writeContextToFile(fs, tt.value, tt.filename)

			// Check error expectation
			if tt.expectErr {
				if err == nil {
					t.Fatal("expected an error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// If the value was empty, filePath should be empty
			if len(tt.value.Raw) == 0 {
				if filePath != "" {
					t.Errorf("expected filePath to be empty, but got: %s", filePath)
				}
				return
			}

			// Verify file path matches expected (sanitized) path
			if tt.expectedFilePath != filePath {
				t.Errorf("Expected %v, but got %v", tt.expectedFilePath, filePath)
			}

			// Read back the file content
			actualContent, err := afero.ReadFile(fs, filePath)
			if err != nil {
				t.Fatal(err)
			}

			// Compare expected and actual file content
			if tt.expectedOutput != string(actualContent) {
				t.Errorf("Expected %v, but got %v", tt.expectedOutput, string(actualContent))
			}
		})
	}
}
