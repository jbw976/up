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

// Package oci contains functions for handling remote oci artifacts
package oci

import (
	"testing"

	"github.com/alecthomas/assert/v2"
)

// TestGetArtifactName tests the GetArtifactName function.
func TestGetArtifactName(t *testing.T) {
	tests := []struct {
		name     string
		artifact string
		expected string
	}{
		{
			name:     "Basic Case",
			artifact: "oci://xpkg.upbound.io/spaces-artifacts/spaces:1.0.0",
			expected: "spaces-1.0.0",
		},
		{
			name:     "No Version",
			artifact: "xpkg.upbound.io/spaces-artifacts/spaces",
			expected: "spaces",
		},
		{
			name:     "Multiple Colons",
			artifact: "oci://xpkg.upbound.io/spaces-artifacts/spaces:1.0.0:latest",
			expected: "spaces-1.0.0-latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetArtifactName(tt.artifact)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestRemoveDomainAndOrg tests the RemoveDomainAndOrg function.
func TestRemoveDomainAndOrg(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		expected string
	}{
		{
			name:     "Basic Case",
			src:      "xpkg.upbound.io/org/repo",
			expected: "repo",
		},
		{
			name:     "Missing Parts",
			src:      "repo",
			expected: "repo",
		},
		{
			name:     "Only Domain",
			src:      "xpkg.upbound.io/repo",
			expected: "repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RemoveDomainAndOrg(tt.src)
			assert.Equal(t, tt.expected, result)
		})
	}
}
