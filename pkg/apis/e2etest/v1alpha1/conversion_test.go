// Copyright 2025 Upbound Inc.
// All rights reserved

package v1alpha1

import (
	"testing"

	"gotest.tools/assert"
)

// TestConvert verifies that Convert correctly filters and converts input to []E2ETest.
func TestConvert(t *testing.T) {
	tests := []struct {
		name     string
		input    []interface{}
		expected []E2ETest
	}{
		{
			name:     "ValidE2ETests",
			input:    []interface{}{E2ETest{}, E2ETest{}},
			expected: []E2ETest{{}, {}},
		},
		{
			name:     "MixedTypes",
			input:    []interface{}{E2ETest{}, "invalid", 123},
			expected: []E2ETest{{}},
		},
		{
			name:     "NoValidTests",
			input:    []interface{}{1, "string", struct{}{}},
			expected: []E2ETest{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _ := Convert(tt.input)
			assert.DeepEqual(t, result, tt.expected)
		})
	}
}
