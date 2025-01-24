// Copyright 2025 Upbound Inc.
// All rights reserved

package test

import (
	"os"
	"testing"

	"github.com/spf13/afero"
	"gotest.tools/v3/assert"
)

// TestDiscoverTestDirectories verifies that discoverTestDirectories correctly finds directories matching glob patterns.
func TestDiscoverTestDirectories(t *testing.T) {
	tests := []struct {
		name        string
		setupFs     func(fs afero.Fs)
		patterns    []string
		testsFolder string
		expectErr   bool
		expected    []string
	}{
		{
			name: "FindAllSubdirectories",
			setupFs: func(fs afero.Fs) {
				_ = fs.MkdirAll("foo", 0o755)
				_ = fs.MkdirAll("bar", 0o755)
			},
			patterns:    []string{"tests/"},
			testsFolder: "tests/",
			expectErr:   false,
			expected:    []string{"bar", "foo"},
		},
		{
			name: "MatchSpecificSubdirectory",
			setupFs: func(fs afero.Fs) {
				_ = fs.MkdirAll("test1", 0o755)
				_ = fs.MkdirAll("test2", 0o755)
			},
			patterns:    []string{"tests/test1"},
			testsFolder: "tests/",
			expectErr:   false,
			expected:    []string{"test1"},
		},
		{
			name: "IgnoreFiles",
			setupFs: func(fs afero.Fs) {
				_ = fs.MkdirAll("foo", 0o755)
				_ = afero.WriteFile(fs, "foo/config.yaml", []byte("content"), os.ModePerm)
			},
			patterns:    []string{"tests/*"},
			testsFolder: "tests/",
			expectErr:   false,
			expected:    []string{"foo"},
		},
		{
			name: "NoMatchingDirectories",
			setupFs: func(fs afero.Fs) {
				_ = fs.MkdirAll("unrelated", 0o755)
			},
			patterns:    []string{"tests/x*"},
			testsFolder: "tests/",
			expectErr:   false,
			expected:    nil,
		},
		{
			name: "MatchNestedDirectories",
			setupFs: func(fs afero.Fs) {
				_ = fs.MkdirAll("xstoragebucket/suite-1", 0o755)
				_ = fs.MkdirAll("xstoragebucket/suite-2", 0o755)
			},
			patterns:    []string{"tests/xstoragebucket/*"},
			testsFolder: "tests/",
			expectErr:   false,
			expected:    []string{"xstoragebucket/suite-1", "xstoragebucket/suite-2"},
		},
		{
			name: "HandleInvalidGlobPattern",
			setupFs: func(fs afero.Fs) {
				_ = fs.MkdirAll("valid", 0o755)
			},
			patterns:    []string{"["}, // Invalid glob pattern
			testsFolder: "tests/",
			expectErr:   true,
			expected:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			tt.setupFs(fs)

			dirs, err := discoverTestDirectories(fs, tt.patterns, tt.testsFolder)

			if tt.expectErr {
				assert.Assert(t, err != nil)
				return
			}

			assert.NilError(t, err)
			assert.DeepEqual(t, dirs, tt.expected)
		})
	}
}
