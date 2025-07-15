// Copyright 2025 Upbound Inc.
// All rights reserved

package crd

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// TestInferProperty tests the inferProperty function.
func TestInferProperty(t *testing.T) {
	type want struct {
		output extv1.JSONSchemaProps
		err    error
	}

	cases := map[string]struct {
		input interface{}
		want  want
	}{
		"StringType": {
			input: "hello",
			want: want{
				output: extv1.JSONSchemaProps{Type: "string"},
				err:    nil,
			},
		},
		"IntegerType": {
			input: 42,
			want: want{
				output: extv1.JSONSchemaProps{Type: "integer"},
				err:    nil,
			},
		},
		"FloatType": {
			input: 3.14,
			want: want{
				output: extv1.JSONSchemaProps{Type: "number"},
				err:    nil,
			},
		},
		"BooleanType": {
			input: true,
			want: want{
				output: extv1.JSONSchemaProps{Type: "boolean"},
				err:    nil,
			},
		},
		"ObjectType": {
			input: map[string]interface{}{
				"key": "value",
			},
			want: want{
				output: extv1.JSONSchemaProps{
					Type: "object",
					Properties: map[string]extv1.JSONSchemaProps{
						"key": {Type: "string"},
					},
				},
				err: nil,
			},
		},
		"ArrayTypeWithElements": {
			input: []interface{}{"one", "two"},
			want: want{
				output: extv1.JSONSchemaProps{
					Type: "array",
					Items: &extv1.JSONSchemaPropsOrArray{
						Schema: &extv1.JSONSchemaProps{Type: "string"},
					},
				},
				err: nil,
			},
		},
		"ArrayTypeEmpty": {
			input: []interface{}{},
			want: want{
				output: extv1.JSONSchemaProps{
					Type: "array",
					Items: &extv1.JSONSchemaPropsOrArray{
						Schema: &extv1.JSONSchemaProps{Type: "object"},
					},
				},
				err: nil,
			},
		},
		"UnknownType": {
			input: nil,
			want: want{
				output: extv1.JSONSchemaProps{Type: "string"},
				err:    nil,
			},
		},
		"ArrayWithMixedTypes": {
			input: []interface{}{1, "2", true},
			want: want{
				output: extv1.JSONSchemaProps{},
				err:    errors.New("mixed types detected in array"),
			},
		},
		"ArrayOfObjectsWithOptionalFields": {
			input: []interface{}{
				map[string]interface{}{
					"name":             "aks-subnet",
					"cidr":             "10.0.1.0/24",
					"serviceEndpoints": []interface{}{"Microsoft.ContainerRegistry"},
				},
				map[string]interface{}{
					"name":             "database-subnet",
					"cidr":             "10.0.2.0/24",
					"delegation":       "Microsoft.DBforMySQL/flexibleServers",
					"serviceEndpoints": []interface{}{"Microsoft.Storage"},
				},
			},
			want: want{
				output: extv1.JSONSchemaProps{
					Type: "array",
					Items: &extv1.JSONSchemaPropsOrArray{
						Schema: &extv1.JSONSchemaProps{
							Type: "object",
							Properties: map[string]extv1.JSONSchemaProps{
								"name": {Type: "string"},
								"cidr": {Type: "string"},
								"serviceEndpoints": {
									Type: "array",
									Items: &extv1.JSONSchemaPropsOrArray{
										Schema: &extv1.JSONSchemaProps{Type: "string"},
									},
								},
								"delegation": {Type: "string"},
							},
						},
					},
				},
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := inferProperty(tc.input)

			// Compare errors
			if err != nil || tc.want.err != nil {
				if err == nil || tc.want.err == nil || err.Error() != tc.want.err.Error() {
					t.Errorf("inferProperty() error = %v, wantErr %v", err, tc.want.err)
				}
				return
			}

			// Compare the output
			if diff := cmp.Diff(got, tc.want.output); diff != "" {
				t.Errorf("inferProperty() -got, +want:\n%s", diff)
			}
		})
	}
}

// TestInferProperties tests the inferProperties function.
func TestInferProperties(t *testing.T) {
	type want struct {
		output map[string]extv1.JSONSchemaProps
		err    error
	}

	cases := map[string]struct {
		input map[string]interface{}
		want  want
	}{
		"SimpleObject": {
			input: map[string]interface{}{
				"key1": "value1",
				"key2": 42,
			},
			want: want{
				output: map[string]extv1.JSONSchemaProps{
					"key1": {Type: "string"},
					"key2": {Type: "integer"},
				},
				err: nil,
			},
		},
		"NestedObject": {
			input: map[string]interface{}{
				"nested": map[string]interface{}{
					"key": true,
				},
			},
			want: want{
				output: map[string]extv1.JSONSchemaProps{
					"nested": {
						Type: "object",
						Properties: map[string]extv1.JSONSchemaProps{
							"key": {Type: "boolean"},
						},
					},
				},
				err: nil,
			},
		},
		"ArrayInObject": {
			input: map[string]interface{}{
				"array": []interface{}{"a", "b"},
			},
			want: want{
				output: map[string]extv1.JSONSchemaProps{
					"array": {
						Type: "array",
						Items: &extv1.JSONSchemaPropsOrArray{
							Schema: &extv1.JSONSchemaProps{Type: "string"},
						},
					},
				},
				err: nil,
			},
		},
		"ObjectWithMixedArray": {
			input: map[string]interface{}{
				"array": []interface{}{1, "2"},
			},
			want: want{
				output: nil,
				err:    errors.New("error inferring property for key 'array': mixed types detected in array"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := InferProperties(tc.input)

			// Compare errors
			if err != nil || tc.want.err != nil {
				if err == nil || tc.want.err == nil || err.Error() != tc.want.err.Error() {
					t.Errorf("inferProperties() error = %v, wantErr %v", err, tc.want.err)
				}
				return
			}

			// Compare the output
			if diff := cmp.Diff(got, tc.want.output); diff != "" {
				t.Errorf("inferProperties() -got, +want:\n%s", diff)
			}
		})
	}
}
