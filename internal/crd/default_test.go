// Copyright 2025 Upbound Inc.
// All rights reserved

package crd

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func TestDefaultValues(t *testing.T) {
	tests := map[string]struct {
		xr   map[string]any
		crd  apiextensionsv1.CustomResourceDefinition
		want map[string]any
	}{
		"ApplyDefaultsToEmptyXR": {
			xr: map[string]any{
				"apiVersion": "example.com/v1",
				"kind":       "ExampleResource",
			},
			crd: apiextensionsv1.CustomResourceDefinition{
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
						{
							Name:    "v1",
							Served:  true,
							Storage: true,
							Schema: &apiextensionsv1.CustomResourceValidation{
								OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
									Properties: map[string]apiextensionsv1.JSONSchemaProps{
										"field1": {
											Type:    "string",
											Default: &apiextensionsv1.JSON{Raw: []byte(`"defaultValue"`)},
										},
									},
								},
							},
						},
						{
							Name:    "v2",
							Served:  true,
							Storage: false, // Ensure we don't pick this one
							Schema: &apiextensionsv1.CustomResourceValidation{
								OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
									Properties: map[string]apiextensionsv1.JSONSchemaProps{
										"field1": {
											Type:    "string",
											Default: &apiextensionsv1.JSON{Raw: []byte(`"wrongValue"`)},
										},
									},
								},
							},
						},
					},
				},
			},
			want: map[string]any{
				"apiVersion": "example.com/v1",
				"kind":       "ExampleResource",
				"field1":     "defaultValue",
			},
		},

		"KeepExistingValues": {
			xr: map[string]any{
				"apiVersion": "example.com/v1",
				"kind":       "ExampleResource",
				"field1":     "existingValue",
			},
			crd: apiextensionsv1.CustomResourceDefinition{
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
						{
							Name:    "v1",
							Served:  true,
							Storage: true,
							Schema: &apiextensionsv1.CustomResourceValidation{
								OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
									Properties: map[string]apiextensionsv1.JSONSchemaProps{
										"field1": {
											Type:    "string",
											Default: &apiextensionsv1.JSON{Raw: []byte(`"defaultValue"`)},
										},
									},
								},
							},
						},
					},
				},
			},
			want: map[string]any{
				"apiVersion": "example.com/v1",
				"kind":       "ExampleResource",
				"field1":     "existingValue", // Should NOT be overwritten
			},
		},

		"VersionNotFound": {
			xr: map[string]any{
				"apiVersion": "example.com/v3", // Doesn't exist in CRD
				"kind":       "ExampleResource",
			},
			crd: apiextensionsv1.CustomResourceDefinition{
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
						{
							Name:    "v1",
							Served:  true,
							Storage: true,
						},
					},
				},
			},
			want: nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := DefaultValues(tc.xr, tc.crd)

			if tc.want == nil {
				if err == nil {
					t.Fatalf("Expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("DefaultValues() returned an error: %v", err)
			}

			if diff := cmp.Diff(tc.xr, tc.want); diff != "" {
				t.Errorf("DefaultValues() mismatch (-got +want):\n%s", diff)
			}
		})
	}
}
