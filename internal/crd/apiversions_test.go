// Copyright 2025 Upbound Inc.
// All rights reserved

package crd

import "testing"

func TestIsKnownAPIVersion(t *testing.T) {
	KnownAPIVersions = []string{"v1", "v1alpha1", "v1beta1"}

	tests := []struct {
		name     string
		segment  string
		expected bool
	}{
		{
			name:     "KnownVersionV1",
			segment:  "v1",
			expected: true,
		},
		{
			name:     "KnownVersionV1alpha1",
			segment:  "v1alpha1",
			expected: true,
		},
		{
			name:     "UnknownVersion",
			segment:  "v2alpha1",
			expected: false,
		},
		{
			name:     "EmptySegment",
			segment:  "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsKnownAPIVersion(tt.segment)
			if got != tt.expected {
				t.Errorf("isKnownAPIVersion() = %v, want %v", got, tt.expected)
			}
		})
	}
}
