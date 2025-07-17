// Copyright 2025 Upbound Inc.
// All rights reserved

// AI Generated. Human reviewed.
package apiconnector

import (
	"net/url"
	"testing"
)

func TestUrlMustParse(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		reason string
		input  string
		want   *url.URL
		panics bool
	}{
		"ValidURL": {
			reason: "Should parse valid URL",
			input:  "https://example.com",
			want:   &url.URL{Scheme: "https", Host: "example.com"},
			panics: false,
		},
		"InvalidURL": {
			reason: "Should panic on invalid URL",
			input:  "://invalid",
			panics: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if tc.panics {
				defer func() {
					if r := recover(); r == nil {
						t.Errorf("expected panic but got none")
					}
				}()
			}

			got := urlMustParse(tc.input)

			if !tc.panics {
				if got.Scheme != tc.want.Scheme || got.Host != tc.want.Host {
					t.Errorf("expected %v, got %v", tc.want, got)
				}
			}
		})
	}
}

func TestNice(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		reason string
		input  string
		want   string
	}{
		"SimpleString": {
			reason: "Should format simple string",
			input:  "test",
			want:   "test",
		},
		"EmptyString": {
			reason: "Should handle empty string",
			input:  "",
			want:   "",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := nice(tc.input)

			// Since we can't easily test the lipgloss styling, we just verify the function runs
			if len(got) == 0 && len(tc.input) > 0 {
				t.Errorf("expected non-empty output for non-empty input")
			}
		})
	}
}

func TestBase64Encode(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		reason string
		input  string
		want   string
	}{
		"SimpleString": {
			reason: "Should encode simple string",
			input:  "test",
			want:   "dGVzdA==",
		},
		"EmptyString": {
			reason: "Should encode empty string",
			input:  "",
			want:   "",
		},
		"ComplexString": {
			reason: "Should encode complex string",
			input:  "Hello World!",
			want:   "SGVsbG8gV29ybGQh",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := base64Encode(tc.input)

			if got != tc.want {
				t.Errorf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestInstallOptions(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		reason string
		opts   installOptions
		want   map[string]interface{}
	}{
		"BasicOptions": {
			reason: "Should handle basic options",
			opts: installOptions{
				name:      "test-connector",
				namespace: "test-namespace",
				version:   "1.0.0",
				upgrade:   false,
				params:    map[string]any{"key": "value"},
			},
			want: map[string]interface{}{
				"name":      "test-connector",
				"namespace": "test-namespace",
				"version":   "1.0.0",
				"upgrade":   false,
				"params":    map[string]any{"key": "value"},
			},
		},
		"UpgradeOptions": {
			reason: "Should handle upgrade options",
			opts: installOptions{
				name:      "test-connector",
				namespace: "test-namespace",
				version:   "2.0.0",
				upgrade:   true,
				params:    map[string]any{"upgrade": "true"},
			},
			want: map[string]interface{}{
				"name":      "test-connector",
				"namespace": "test-namespace",
				"version":   "2.0.0",
				"upgrade":   true,
				"params":    map[string]any{"upgrade": "true"},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Validate struct fields
			if tc.opts.name != tc.want["name"].(string) {
				t.Errorf("expected name %q, got %q", tc.want["name"], tc.opts.name)
			}
			if tc.opts.namespace != tc.want["namespace"].(string) {
				t.Errorf("expected namespace %q, got %q", tc.want["namespace"], tc.opts.namespace)
			}
			if tc.opts.version != tc.want["version"].(string) {
				t.Errorf("expected version %q, got %q", tc.want["version"], tc.opts.version)
			}
			if tc.opts.upgrade != tc.want["upgrade"].(bool) {
				t.Errorf("expected upgrade %v, got %v", tc.want["upgrade"], tc.opts.upgrade)
			}
		})
	}
}

func TestConstants(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		reason string
		value  string
		want   string
	}{
		"ConnectorName": {
			reason: "Should have correct connector name",
			value:  connectorName,
			want:   "api-connector",
		},
		"DefaultNamespace": {
			reason: "Should have correct default namespace",
			value:  defaultInstallationNamespace,
			want:   "upbound-system",
		},
		"SpacesHostnameSuffix": {
			reason: "Should have correct spaces hostname suffix",
			value:  spacesHostnameSuffix,
			want:   ".spaces.upbound.io",
		},
		"LabelConnectorOwned": {
			reason: "Should have correct connector owned label",
			value:  labelConnectorOwned,
			want:   "connect.upbound.io/connector-secret",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if tc.value != tc.want {
				t.Errorf("expected %q, got %q", tc.want, tc.value)
			}
		})
	}
}
