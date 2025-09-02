// Copyright 2025 Upbound Inc.
// All rights reserved

package v1alpha1

import (
	"testing"

	"gotest.tools/v3/assert"
	runtime "k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
)

// TestConvert verifies that Convert correctly filters and converts input to []OperationTest.
func TestConvert(t *testing.T) {
	tests := []struct {
		name     string
		input    []interface{}
		expected []OperationTest
	}{
		{
			name:     "ValidOperationTests",
			input:    []interface{}{OperationTest{}, OperationTest{}},
			expected: []OperationTest{{}, {}},
		},
		{
			name:  "MixedTypes",
			input: []interface{}{OperationTest{}, "invalid", 123},
			expected: []OperationTest{
				{}, // Only the valid OperationTest instance should remain
			},
		},
		{
			name:     "NoValidTests",
			input:    []interface{}{1, "string", struct{}{}},
			expected: nil, // No valid tests should be returned
		},
		{
			name: "InvalidOperationTest",
			input: []interface{}{
				OperationTest{
					Spec: OperationTestSpec{
						RequiredResources: []runtime.RawExtension{
							{Raw: []byte("test")},
						},
						RequiredResourcesPath: "path",
					},
				},
			}, // Invalid due to mutually exclusive fields
			expected: []OperationTest{}, // Should be excluded due to validation failure
		},
		{
			name: "ValidWithRequiredResourcesOnly",
			input: []interface{}{
				OperationTest{
					Spec: OperationTestSpec{
						RequiredResources: []runtime.RawExtension{
							{Raw: []byte(`{"test": "data"}`)},
						},
					},
				},
			},
			expected: []OperationTest{
				{
					Spec: OperationTestSpec{
						RequiredResources: []runtime.RawExtension{
							{Raw: []byte(`{"test": "data"}`)},
						},
					},
				},
			},
		},
		{
			name: "ValidWithRequiredResourcesPathOnly",
			input: []interface{}{
				OperationTest{
					Spec: OperationTestSpec{
						RequiredResourcesPath: "resources/path",
					},
				},
			},
			expected: []OperationTest{
				{
					Spec: OperationTestSpec{
						RequiredResourcesPath: "resources/path",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Convert(tt.input)

			if len(tt.expected) == 0 {
				assert.Assert(t, err != nil, "Expected an error but got nil")
				assert.Assert(t, result == nil, "Expected nil result but got non-nil")
			} else {
				assert.Assert(t, err == nil || errors.Is(err, nil), "Expected no error but got one")
				assert.DeepEqual(t, result, tt.expected)
			}
		})
	}
}
