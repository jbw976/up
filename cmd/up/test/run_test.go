// Copyright 2025 Upbound Inc.
// All rights reserved

// Package test contains commands for working with tests project.
package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pterm/pterm"
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/yaml"
)

func TestAssertions(t *testing.T) {
	tests := []struct {
		name           string
		output         string
		expectedYAML   []string
		expectErr      bool
		expectedErrMsg string
	}{
		{
			name: "ValidMatchingManifest",
			output: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key: value
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
spec:
  replicas: 2
`,
			expectedYAML: []string{
				`
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key: value
`,
			},
			expectErr: false,
		},
		{
			name: "MissingExpectedResource",
			output: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key: value
`,
			expectedYAML: []string{
				`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: missing-deployment
`,
			},
			expectErr:      true,
			expectedErrMsg: "no actual resource found",
		},
	}

	ctx := context.TODO()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var expectedAssertions []runtime.RawExtension

			// Convert expected YAML into runtime.RawExtensions
			for _, yamlStr := range tt.expectedYAML {
				var expectedObj map[string]interface{}
				err := yaml.Unmarshal([]byte(yamlStr), &expectedObj)
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				expectedJSON, err := json.Marshal(expectedObj)
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				expectedAssertions = append(expectedAssertions, runtime.RawExtension{Raw: expectedJSON})
			}

			err := assertions(ctx, tt.output, "test", expectedAssertions, nil)

			if tt.expectErr {
				if err == nil {
					t.Fatal("expected an error but got nil")
				}

				if !strings.Contains(err.Error(), tt.expectedErrMsg) {
					t.Errorf("Expected error to contain %q, but got %q", tt.expectedErrMsg, err.Error())
				}
			} else if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

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

// captureOutput captures printed output from pterm.
func captureOutput(f func()) string {
	// Create a buffer to capture output.
	var buf bytes.Buffer
	writer := &buf

	// Set pterm output to the buffer.
	pterm.SetDefaultOutput(writer)

	// Execute the function while capturing output.
	f()

	// Reset pterm output (Avoid using nil, as it will cause a panic).
	pterm.SetDefaultOutput(writer)

	// Normalize output (trim extra spaces and fix line breaks).
	return strings.TrimSpace(strings.ReplaceAll(buf.String(), " \n", "\n"))
}

func TestDisplayTestResults(t *testing.T) {
	tests := []struct {
		name     string
		ttotal   int
		tsuccess int
		terr     int
		expected string
	}{
		{
			name:     "AllTestsPassed",
			ttotal:   5,
			tsuccess: 5,
			terr:     0,
			//nolint:dupword // our return
			expected: `SUCCESS:
SUCCESS: Tests Summary:
SUCCESS: ------------------
SUCCESS: Total Tests Executed: 5
SUCCESS: Passed tests:         5
SUCCESS: Failed tests:         0`,
		},
		{
			name:     "SomeTestsFailed",
			ttotal:   5,
			tsuccess: 3,
			terr:     2,
			//nolint:dupword // our return
			expected: `ERROR:
ERROR: Tests Summary:
ERROR: ------------------
ERROR: Total Tests Executed: 5
ERROR: Passed tests:         3
ERROR: Failed tests:         2`,
		},
		{
			name:     "AllTestsFailed",
			ttotal:   4,
			tsuccess: 0,
			terr:     4,
			//nolint:dupword // our return
			expected: `ERROR:
ERROR: Tests Summary:
ERROR: ------------------
ERROR: Total Tests Executed: 4
ERROR: Passed tests:         0
ERROR: Failed tests:         4`,
		},
		{
			name:     "NoTestsExecuted",
			ttotal:   0,
			tsuccess: 0,
			terr:     0,
			//nolint:dupword // our return
			expected: `SUCCESS:
SUCCESS: Tests Summary:
SUCCESS: ------------------
SUCCESS: Total Tests Executed: 0
SUCCESS: Passed tests:         0
SUCCESS: Failed tests:         0`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureOutput(func() {
				displayTestResults(tt.ttotal, tt.tsuccess, tt.terr)
			})

			if tt.expected != output {
				t.Errorf("Expected %v, but got %v", tt.expected, output)
			}
		})
	}
}

func TestSetEnvVars(t *testing.T) {
	tests := []struct {
		name      string
		inputVars map[string]string
		expectErr bool
	}{
		{
			name: "SetSingleEnvVar",
			inputVars: map[string]string{
				"TEST_ENV": "test_value",
			},
			expectErr: false,
		},
		{
			name: "SetMultipleEnvVars",
			inputVars: map[string]string{
				"ENV_ONE": "value1",
				"ENV_TWO": "value2",
			},
			expectErr: false,
		},
		{
			name:      "NoEnvVars", // Should handle empty map
			inputVars: map[string]string{},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call setEnvVars
			cleanup, err := setEnvVars(tt.inputVars)

			// Check for expected errors
			if tt.expectErr {
				if err == nil {
					t.Fatal("expected an error but got nil")
				}

				return
			}

			if err != nil {
				t.Fatal(err)
			}
			if cleanup == nil {
				t.Fatal("expected cleanup to be not nil, but got nil")
			}

			// Verify environment variables were set correctly
			for key, expectedValue := range tt.inputVars {
				actualValue := os.Getenv(key)
				if expectedValue != actualValue {
					t.Errorf("Expected %v, but got %v", expectedValue, actualValue)
				}
			}
			// Cleanup (Unset environment variables)
			cleanup()

			// Verify that variables are unset
			for key := range tt.inputVars {
				actualValue := os.Getenv(key)
				if actualValue != "" {
					t.Errorf("expected actualValue to be empty, but got: %s", actualValue)
				}
			}
		})
	}
}

// MockClientConfig implements clientcmd.ClientConfig.
type MockClientConfig struct {
	rawConfig api.Config
	err       error
}

// RawConfig returns the mock kubeconfig.
func (m *MockClientConfig) RawConfig() (api.Config, error) {
	return m.rawConfig, m.err
}

// ClientConfig (unused in this test).
func (m *MockClientConfig) ClientConfig() (*rest.Config, error) {
	return nil, m.err
}

// Namespace (unused in this test).
func (m *MockClientConfig) Namespace() (string, bool, error) {
	return "", false, m.err
}

// ConfigAccess (unused in this test).
func (m *MockClientConfig) ConfigAccess() clientcmd.ConfigAccess {
	return nil
}

func TestWriteClientConfig(t *testing.T) {
	tests := []struct {
		name           string
		clientConfig   clientcmd.ClientConfig
		expectErr      bool
		expectedErrMsg string
	}{
		{
			name: "ValidKubeconfig",
			clientConfig: &MockClientConfig{
				rawConfig: api.Config{
					Clusters: map[string]*api.Cluster{
						"test-cluster": {Server: "https://127.0.0.1:6443"},
					},
				},
				err: nil,
			},
			expectErr: false,
		},
		{
			name: "ClientConfigError",
			clientConfig: &MockClientConfig{
				rawConfig: api.Config{},
				err:       fmt.Errorf("mock error retrieving config"),
			},
			expectErr:      true,
			expectedErrMsg: "failed to get raw config from clientcmd.ClientConfig",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a real temporary directory
			tmpDir := t.TempDir()
			kubeconfigPath, err := writeClientConfig(tt.clientConfig, tmpDir)

			if tt.expectErr {
				if err == nil {
					t.Fatal("expected an error but got nil")
				}
				if !strings.Contains(err.Error(), tt.expectedErrMsg) {
					t.Errorf("Expected error to contain %q, but got %q", tt.expectedErrMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatal(err)
			}
			if kubeconfigPath == "" {
				t.Fatal("expected kubeconfigPath to be not empty, but got an empty string")
			}

			if filepath.Join(tmpDir, "kubeconfig.yaml") != kubeconfigPath {
				t.Errorf("Expected %v, but got %v", filepath.Join(tmpDir, "kubeconfig.yaml"), kubeconfigPath)
			}

			// Verify file exists
			_, err = os.Stat(kubeconfigPath)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestIsMatchingManifest(t *testing.T) {
	tests := []struct {
		name                string
		expected            unstructured.Unstructured
		rendered            unstructured.Unstructured
		expectedAnnotations map[string]string
		expectMatch         bool
	}{
		{
			name: "MatchingAll",
			expected: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "ec2.aws.upbound.io/v1beta1",
					"kind":       "SecurityGroupRule",
					"metadata": map[string]interface{}{
						"name": "test",
						"annotations": map[string]interface{}{
							"crossplane.io/composition-resource-name": "test",
						},
					},
				},
			},
			rendered: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "ec2.aws.upbound.io/v1beta1",
					"kind":       "SecurityGroupRule",
					"metadata": map[string]interface{}{
						"name": "test",
						"annotations": map[string]interface{}{
							"crossplane.io/composition-resource-name": "test",
						},
					},
				},
			},
			expectedAnnotations: map[string]string{
				"crossplane.io/composition-resource-name": "test",
			},
			expectMatch: true,
		},
		{
			name: "MatchingWithoutName",
			expected: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "ec2.aws.upbound.io/v1beta1",
					"kind":       "SecurityGroupRule",
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							"crossplane.io/composition-resource-name": "test",
						},
					},
				},
			},
			rendered: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "ec2.aws.upbound.io/v1beta1",
					"kind":       "SecurityGroupRule",
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							"crossplane.io/composition-resource-name": "test",
						},
					},
				},
			},
			expectedAnnotations: map[string]string{
				"crossplane.io/composition-resource-name": "test",
			},
			expectMatch: true,
		},
		{
			name: "MismatchingName",
			expected: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "ec2.aws.upbound.io/v1beta1",
					"kind":       "SecurityGroupRule",
					"metadata": map[string]interface{}{
						"name": "test-configmap",
					},
				},
			},
			rendered: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "ec2.aws.upbound.io/v1beta1",
					"kind":       "SecurityGroupRule",
					"metadata": map[string]interface{}{
						"name": "different-name",
					},
				},
			},
			expectedAnnotations: map[string]string{},
			expectMatch:         false,
		},
		{
			name: "MissingAnnotation",
			expected: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "ec2.aws.upbound.io/v1beta1",
					"kind":       "SecurityGroupRule",
					"metadata": map[string]interface{}{
						"name": "test-configmap",
					},
				},
			},
			rendered: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "ec2.aws.upbound.io/v1beta1",
					"kind":       "SecurityGroupRule",
					"metadata": map[string]interface{}{
						"name": "test-configmap",
					},
				},
			},
			expectedAnnotations: map[string]string{
				"env": "production",
			},
			expectMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if isMatchingManifest(tt.expected, tt.rendered, tt.expectedAnnotations) != tt.expectMatch {
				t.Errorf("Test %q failed: expected match result %v", tt.name, tt.expectMatch)
			}
		})
	}
}

func TestTruncateAndValidateName(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "ShortNameOK",
			prefix:   "cp",
			input:    "webapp",
			expected: "cp-webapp",
			wantErr:  false,
		},
		{
			name:     "Exact63Characters",
			prefix:   "cp",
			input:    "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefgh",
			expected: "cp-abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefgh",
			wantErr:  false,
		},
		{
			name:     "Over63CharactersShouldTruncate",
			prefix:   "cp",
			input:    "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmno",
			expected: "cp-abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefgh",
			wantErr:  false,
		},
		{
			name:     "InvalidCharacters",
			prefix:   "cp",
			input:    "Invalid_Chars",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "EndsWithDashAfterTruncation",
			prefix:   "prefix",
			input:    "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabc-",
			expected: "prefix-abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabc",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := truncateAndValidateName(tt.prefix, tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("Expected error: %v, got: %v", tt.wantErr, err)
			}

			if err == nil {
				if len(got) > 63 {
					t.Errorf("Name too long: got %d characters", len(got))
				}
				if got != tt.expected {
					t.Errorf("Expected name %q, got %q", tt.expected, got)
				}
			}
		})
	}
}
