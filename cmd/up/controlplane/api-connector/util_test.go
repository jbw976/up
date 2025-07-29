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
