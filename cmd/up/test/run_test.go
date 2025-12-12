// Copyright 2025 Upbound Inc.
// All rights reserved

package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/yaml"

	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/upterm"
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

	ctx := t.Context()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var expectedAssertions []runtime.RawExtension

			// Convert expected YAML into runtime.RawExtensions
			for _, yamlStr := range tt.expectedYAML {
				var expectedObj map[string]any
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

			err := assertions(
				ctx,
				tt.output,
				"test",
				expectedAssertions,
				nil,
				upterm.NewTestPrinter(),
			)

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

// captureOutput captures printed output from pterm.
func captureOutput(f func(p upterm.Printer)) string {
	// Create a buffer to capture output.
	var buf bytes.Buffer
	p := upterm.NewPrinter(&buf, &buf, config.FormatDefault, false)

	// Execute the function while capturing output.
	f(p)

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
			output := captureOutput(func(p upterm.Printer) {
				displayTestResults(p, tt.ttotal, tt.tsuccess, tt.terr)
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
				Object: map[string]any{
					"apiVersion": "ec2.aws.upbound.io/v1beta1",
					"kind":       "SecurityGroupRule",
					"metadata": map[string]any{
						"name": "test",
						"annotations": map[string]any{
							"crossplane.io/composition-resource-name": "test",
						},
					},
				},
			},
			rendered: unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "ec2.aws.upbound.io/v1beta1",
					"kind":       "SecurityGroupRule",
					"metadata": map[string]any{
						"name": "test",
						"annotations": map[string]any{
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
				Object: map[string]any{
					"apiVersion": "ec2.aws.upbound.io/v1beta1",
					"kind":       "SecurityGroupRule",
					"metadata": map[string]any{
						"annotations": map[string]any{
							"crossplane.io/composition-resource-name": "test",
						},
					},
				},
			},
			rendered: unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "ec2.aws.upbound.io/v1beta1",
					"kind":       "SecurityGroupRule",
					"metadata": map[string]any{
						"annotations": map[string]any{
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
				Object: map[string]any{
					"apiVersion": "ec2.aws.upbound.io/v1beta1",
					"kind":       "SecurityGroupRule",
					"metadata": map[string]any{
						"name": "test-configmap",
					},
				},
			},
			rendered: unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "ec2.aws.upbound.io/v1beta1",
					"kind":       "SecurityGroupRule",
					"metadata": map[string]any{
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
				Object: map[string]any{
					"apiVersion": "ec2.aws.upbound.io/v1beta1",
					"kind":       "SecurityGroupRule",
					"metadata": map[string]any{
						"name": "test-configmap",
					},
				},
			},
			rendered: unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "ec2.aws.upbound.io/v1beta1",
					"kind":       "SecurityGroupRule",
					"metadata": map[string]any{
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
