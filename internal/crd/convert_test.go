// Copyright 2025 Upbound Inc.
// All rights reserved

package crd

import (
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kube-openapi/pkg/spec3"
	"k8s.io/kube-openapi/pkg/validation/spec"
	"sigs.k8s.io/yaml"

	_ "embed"
)

//go:embed testdata/template.fn.crossplane.io_kclinputs.yaml
var testCRD []byte

func TestFilesToOpenAPI(t *testing.T) {
	t.Parallel()

	// Define test cases
	tests := []struct {
		name        string
		crdContent  []byte
		expectedErr bool
	}{
		{
			name:        "ValidCRDFromEmbed",
			crdContent:  testCRD, // using the embedded CRD file
			expectedErr: false,
		},
		{
			name:        "InvalidCRD",
			crdContent:  []byte(`invalid: crd content`),
			expectedErr: true,
		},
		{
			name: "CRDMissingVersion",
			crdContent: []byte(`
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: testresources.testgroup.example.com
spec:
  group: testgroup.example.com
  versions: []
  names:
    kind: TestResource
    plural: testresources
`),
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Use an in-memory filesystem
			fs := afero.NewMemMapFs()

			// Call ConvertToOpenAPI
			outputPaths, err := FilesToOpenAPI(fs, tt.crdContent, "test-crd.yaml")

			// Check if an error was expected
			if tt.expectedErr {
				assert.Assert(t, err != nil)
				return
			}
			assert.NilError(t, err)
			assert.Assert(t, cmp.Len(outputPaths, 1))

			// Perform validation for the success case (only needed if no error was expected)
			_, err = afero.Exists(fs, filepath.Join(outputPaths[0], "template_fn_crossplane_io_v1beta1_kclinput.yaml"))
			assert.NilError(t, err)

			// Read the content from the file in-memory
			output, err := afero.ReadFile(fs, outputPaths[0])
			assert.NilError(t, err)

			var openapi *spec3.OpenAPI
			err = yaml.Unmarshal(output, &openapi)
			assert.NilError(t, err)

			apiVersionDefault := openapi.Components.Schemas["io.crossplane.fn.template.v1beta1.KCLInput"].SchemaProps.Properties["apiVersion"].Default
			assert.Equal(t, "template.fn.crossplane.io/v1beta1", apiVersionDefault, "The default value of apiVersion does not match the expected content")

			kindDefault := openapi.Components.Schemas["io.crossplane.fn.template.v1beta1.KCLInput"].SchemaProps.Properties["kind"].Default
			assert.Equal(t, "KCLInput", kindDefault, "The default value of kind does not match the expected content")
		})
	}
}

func TestAddDefaultAPIVersionAndKind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		initialSchema      spec.Schema
		gvk                schema.GroupVersionKind
		expectedAPIVersion string
		expectedKind       string
	}{
		{
			name: "ApiVersionAndKind",
			initialSchema: spec.Schema{
				SchemaProps: spec.SchemaProps{
					Properties: map[string]spec.Schema{
						"apiVersion": {},
						"kind":       {},
					},
				},
			},
			gvk:                schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "ExampleKind"},
			expectedAPIVersion: "example.com/v1",
			expectedKind:       "ExampleKind",
		},
		{
			name: "ApiVersion",
			initialSchema: spec.Schema{
				SchemaProps: spec.SchemaProps{
					Properties: map[string]spec.Schema{
						"apiVersion": {},
					},
				},
			},
			gvk:                schema.GroupVersionKind{Group: "example.com", Version: "v2", Kind: "AnotherKind"},
			expectedAPIVersion: "example.com/v2",
			expectedKind:       "",
		},
		{
			name: "Kind",
			initialSchema: spec.Schema{
				SchemaProps: spec.SchemaProps{
					Properties: map[string]spec.Schema{
						"kind": {},
					},
				},
			},
			gvk:                schema.GroupVersionKind{Group: "example.com", Version: "v1alpha1", Kind: "SampleKind"},
			expectedAPIVersion: "",
			expectedKind:       "SampleKind",
		},
		{
			name: "Nothing",
			initialSchema: spec.Schema{
				SchemaProps: spec.SchemaProps{
					Properties: map[string]spec.Schema{},
				},
			},
			gvk:                schema.GroupVersionKind{Group: "example.com", Version: "v1beta1", Kind: "NoDefaults"},
			expectedAPIVersion: "",
			expectedKind:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Call the function to add default values
			addDefaultAPIVersionAndKind(&tt.initialSchema, tt.gvk)

			// Check the apiVersion default if the property exists
			if prop, ok := tt.initialSchema.Properties["apiVersion"]; ok {
				assert.Equal(t, tt.expectedAPIVersion, prop.Default, "apiVersion default should match expected value")
			}

			// Check the kind default if the property exists
			if prop, ok := tt.initialSchema.Properties["kind"]; ok {
				assert.Equal(t, tt.expectedKind, prop.Default, "kind default should match expected value")
			}
		})
	}
}

var testValidCRD = []byte(`
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: objects.kubernetes.crossplane.io
spec:
  group: kubernetes.crossplane.io
  names:
    kind: Object
    plural: objects
    singular: object
  scope: Cluster
  versions:
    - name: v1alpha1
      schema:
        openAPIV3Schema:
          properties:
            spec:
              properties:
                forProvider:
                  properties:
                    manifest:
                      type: object
                      x-kubernetes-embedded-resource: true
                      x-kubernetes-preserve-unknown-fields: true
`)

func TestModifyCRDManifestFields(t *testing.T) {
	t.Parallel()

	// Define test cases
	tests := []struct {
		name        string
		crdContent  []byte
		expectedErr bool
	}{
		{
			name:        "ValidCRD",
			crdContent:  testValidCRD,
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Unmarshal CRD from YAML
			var crd extv1.CustomResourceDefinition
			err := yaml.Unmarshal(tt.crdContent, &crd)
			if err != nil {
				if tt.expectedErr {
					t.Logf("Expected failure due to invalid YAML: %v", err)
					return
				}
				t.Fatalf("Failed to unmarshal CRD: %v", err)
			}

			// Call function to modify CRD
			modifyCRDManifestFields(&crd)

			// Check if an error was expected
			if tt.expectedErr {
				assert.Assert(t, err != nil, "Expected an error but got none")
				return
			}
			assert.NilError(t, err)

			// Validate modifications: manifest should have XEmbeddedResource=false, XPreserveUnknownFields=nil
			modifiedManifest := crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"].
				Properties["forProvider"].Properties["manifest"]

			assert.Equal(t, modifiedManifest.XEmbeddedResource, false, "Expected XEmbeddedResource to be false")
			assert.Assert(t, modifiedManifest.XPreserveUnknownFields == nil, "Expected XPreserveUnknownFields to be nil")
			assert.Equal(t, modifiedManifest.Type, "object", "Expected Type to be 'object'")
		})
	}
}

func TestUpdateSchemaPropertiesXEmbeddedResource(t *testing.T) {
	tests := []struct {
		name     string
		input    *extv1.JSONSchemaProps
		expected *extv1.JSONSchemaProps
	}{
		{
			name:     "NilSchema",
			input:    nil,
			expected: nil,
		},
		{
			name: "SchemaWithXEmbeddedResourceAndXPreserveUnknownFields",
			input: &extv1.JSONSchemaProps{
				XEmbeddedResource:      true,
				XPreserveUnknownFields: &[]bool{true}[0],
			},
			expected: &extv1.JSONSchemaProps{
				XEmbeddedResource:      false,
				XPreserveUnknownFields: nil,
				Type:                   "object",
				AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
					Allows: true,
					Schema: nil,
				},
			},
		},
		{
			name: "NestedProperties",
			input: &extv1.JSONSchemaProps{
				Properties: map[string]extv1.JSONSchemaProps{
					"nested": {
						XEmbeddedResource:      true,
						XPreserveUnknownFields: &[]bool{true}[0],
					},
				},
			},
			expected: &extv1.JSONSchemaProps{
				Properties: map[string]extv1.JSONSchemaProps{
					"nested": {
						XEmbeddedResource:      false,
						XPreserveUnknownFields: nil,
						Type:                   "object",
						AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
							Allows: true,
							Schema: nil,
						},
					},
				},
			},
		},
		{
			name: "AdditionalProperties",
			input: &extv1.JSONSchemaProps{
				AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
					Schema: &extv1.JSONSchemaProps{
						XEmbeddedResource:      true,
						XPreserveUnknownFields: &[]bool{true}[0],
					},
				},
			},
			expected: &extv1.JSONSchemaProps{
				AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
					Schema: &extv1.JSONSchemaProps{
						XEmbeddedResource:      false,
						XPreserveUnknownFields: nil,
						Type:                   "object",
						AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
							Allows: true,
							Schema: nil,
						},
					},
				},
			},
		},
		{
			name: "Items",
			input: &extv1.JSONSchemaProps{
				Items: &extv1.JSONSchemaPropsOrArray{
					Schema: &extv1.JSONSchemaProps{
						XEmbeddedResource:      true,
						XPreserveUnknownFields: &[]bool{true}[0],
					},
				},
			},
			expected: &extv1.JSONSchemaProps{
				Items: &extv1.JSONSchemaPropsOrArray{
					Schema: &extv1.JSONSchemaProps{
						XEmbeddedResource:      false,
						XPreserveUnknownFields: nil,
						Type:                   "object",
						AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
							Allows: true,
							Schema: nil,
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updateSchemaPropertiesXEmbeddedResource(tt.input)
			if !compareJSONSchemaProps(tt.input, tt.expected) {
				t.Errorf("updateSchemaPropertiesXEmbeddedResource() = %v, want %v", tt.input, tt.expected)
			}
		})
	}
}

// Helper function to compare two JSONSchemaProps.
func compareJSONSchemaProps(a, b *extv1.JSONSchemaProps) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.XEmbeddedResource != b.XEmbeddedResource {
		return false
	}
	if (a.XPreserveUnknownFields == nil) != (b.XPreserveUnknownFields == nil) {
		return false
	}
	if a.XPreserveUnknownFields != nil && b.XPreserveUnknownFields != nil && *a.XPreserveUnknownFields != *b.XPreserveUnknownFields {
		return false
	}
	if a.Type != b.Type {
		return false
	}
	if !compareJSONSchemaPropsOrBool(a.AdditionalProperties, b.AdditionalProperties) {
		return false
	}
	if len(a.Properties) != len(b.Properties) {
		return false
	}
	for key, aProp := range a.Properties {
		bProp, ok := b.Properties[key]
		if !ok {
			return false
		}
		if !compareJSONSchemaProps(&aProp, &bProp) {
			return false
		}
	}
	return compareJSONSchemaPropsOrArray(a.Items, b.Items)
}

// Helper function to compare two JSONSchemaPropsOrBool.
func compareJSONSchemaPropsOrBool(a, b *extv1.JSONSchemaPropsOrBool) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.Allows != b.Allows {
		return false
	}
	return compareJSONSchemaProps(a.Schema, b.Schema)
}

// Helper function to compare two JSONSchemaPropsOrArray.
func compareJSONSchemaPropsOrArray(a, b *extv1.JSONSchemaPropsOrArray) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if !compareJSONSchemaProps(a.Schema, b.Schema) {
		return false
	}
	if len(a.JSONSchemas) != len(b.JSONSchemas) {
		return false
	}
	for i := range a.JSONSchemas {
		if !compareJSONSchemaProps(&a.JSONSchemas[i], &b.JSONSchemas[i]) {
			return false
		}
	}
	return true
}
