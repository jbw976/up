// Copyright 2025 Upbound Inc.
// All rights reserved

package helm

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestParse(t *testing.T) {
	cases := map[string]struct {
		reason string
		parser *Parser
		params map[string]any
		err    error
	}{
		"SuccessfulBaseNoOverrides": {
			reason: "If no overrides are provided the base should be returned.",
			parser: &Parser{
				values: map[string]any{
					"test": "value",
				},
			},
			params: map[string]any{
				"test": "value",
			},
		},
		"SuccessfulBaseWithOverrides": {
			reason: "If base and overrides are provided then overrides should take precedence.",
			parser: &Parser{
				values: map[string]any{
					"test": "value",
					"other": map[string]any{
						"nested": "something",
					},
				},
				overrides: map[string]string{
					"other.nested": "somethingElse",
				},
			},
			params: map[string]any{
				"test": "value",
				"other": map[string]any{
					"nested": "somethingElse",
				},
			},
		},
		"SuccessfulOverrides": {
			reason: "If no base is provided just overrides should be returned.",
			parser: &Parser{
				values: map[string]any{},
				overrides: map[string]string{
					"other.nested": "somethingElse",
				},
			},
			params: map[string]any{
				"other": map[string]any{
					"nested": "somethingElse",
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p, err := tc.parser.Parse()
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nParse(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.params, p); diff != "" {
				t.Errorf("\n%s\nParse(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
