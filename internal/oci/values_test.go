// Copyright 2025 Upbound Inc.
// All rights reserved

// Package oci contains functions for handling remote oci artifacts
package oci

import (
	"fmt"
	"os"
	"testing"

	"github.com/alecthomas/assert/v2"
	"helm.sh/helm/v3/pkg/chart"
)

// MockPathNavigator is a mock implementation of PathNavigator for testing.
type MockPathNavigator struct {
	Extractors []string `json:"extractors"`
}

func (m *MockPathNavigator) Extractor() ([]string, error) {
	return m.Extractors, nil
}

// Mock functions.
func mockLoaderLoad(name string) (*chart.Chart, error) {
	return &chart.Chart{
		Metadata: &chart.Metadata{
			Name: name,
		},
		Values: map[string]interface{}{
			"supportedVersions": []string{"v1", "v2", "v3"},
		},
	}, nil
}

func mockPullRun(_, _, _, _ string) (string, error) {
	dir, err := os.MkdirTemp("", "mock")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}
	return dir, nil
}

func TestGetValuesFromChartWithLoaderAndPull(t *testing.T) {
	tests := []struct {
		name          string
		chartName     string
		version       string
		mockNavigator *MockPathNavigator
		mockLoader    func(name string) (*chart.Chart, error)
		mockPull      func(_, _, _, _ string) (string, error)
		username      string
		password      string
		expected      []string
		expectedErr   string
	}{
		{
			name:      "Success",
			chartName: "test-chart",
			version:   "1.0.0",
			mockNavigator: &MockPathNavigator{
				Extractors: []string{"v1", "v2", "v3"},
			},
			mockLoader: mockLoaderLoad,
			mockPull:   mockPullRun,
			username:   "",
			password:   "",
			expected:   []string{"v1", "v2", "v3"},
		},
		{
			name:          "PullError",
			chartName:     "test-chart",
			version:       "1.0.0",
			mockNavigator: &MockPathNavigator{},
			mockLoader:    mockLoaderLoad,
			mockPull: func(_, _, _, _ string) (string, error) {
				return "", fmt.Errorf("failed to pull chart")
			},
			username:    "",
			password:    "",
			expectedErr: "failed to pull chart",
		},
		{
			name:          "LoadError",
			chartName:     "test-chart",
			version:       "1.0.0",
			mockNavigator: &MockPathNavigator{},
			mockLoader: func(_ string) (*chart.Chart, error) {
				return nil, fmt.Errorf("failed to load chart")
			},
			mockPull:    mockPullRun,
			username:    "",
			password:    "",
			expectedErr: "failed to load chart",
		},
		{
			name:          "JSONError",
			chartName:     "test-chart",
			version:       "1.0.0",
			mockNavigator: &MockPathNavigator{},
			mockLoader: func(name string) (*chart.Chart, error) {
				return &chart.Chart{
					Metadata: &chart.Metadata{
						Name: name,
					},
					Values: map[string]interface{}{
						"supportedVersions": func() {}, // Invalid type for JSON marshaling
					},
				}, nil
			},
			mockPull:    mockPullRun,
			username:    "",
			password:    "",
			expectedErr: "failed to marshal chart values",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			versions, err := getValuesFromChartWithLoaderAndPull(tt.chartName, tt.version, tt.mockNavigator, tt.mockLoader, tt.mockPull, tt.username, tt.password)
			if tt.expectedErr != "" {
				if err == nil {
					t.Fatalf("expected error: %v, got none", tt.expectedErr)
				}
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				assert.Equal(t, tt.expected, versions)
			}
		})
	}
}
