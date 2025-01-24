// Copyright 2025 Upbound Inc.
// All rights reserved

package v1alpha1

import (
	"testing"

	"gotest.tools/assert"
	runtime "k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// TestConvert verifies that Convert correctly filters and converts input to []CompositionTest.
func TestConvert(t *testing.T) {
	tests := []struct {
		name     string
		input    []interface{}
		expected []CompositionTest
	}{
		{
			name:     "ValidCompositionTests",
			input:    []interface{}{CompositionTest{}, CompositionTest{}},
			expected: []CompositionTest{{}, {}},
		},
		{
			name:  "MixedTypes",
			input: []interface{}{CompositionTest{}, "invalid", 123},
			expected: []CompositionTest{
				{}, // Only the valid CompositionTest instance should remain
			},
		},
		{
			name:     "NoValidTests",
			input:    []interface{}{1, "string", struct{}{}},
			expected: nil, // No valid tests should be returned
		},
		{
			name: "InvalidCompositionTest",
			input: []interface{}{
				CompositionTest{
					Spec: CompositionTestSpec{
						XR: runtime.RawExtension{
							Raw: []byte("test"),
						},
						XRPath: "path",
					},
				},
			}, // Invalid due to mutually exclusive fields
			expected: []CompositionTest{}, // Should be excluded due to validation failure
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
