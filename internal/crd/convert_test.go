// Copyright 2024 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package crd

import (
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kube-openapi/pkg/spec3"
	"k8s.io/kube-openapi/pkg/validation/spec"
	"sigs.k8s.io/yaml"

	_ "embed"
)

//go:embed testdata/template.fn.crossplane.io_kclinputs.yaml
var testCRD []byte

func TestConvertToOpenAPI(t *testing.T) {
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
			// Use an in-memory filesystem
			fs := afero.NewMemMapFs()

			// Call ConvertToOpenAPI
			outputPath, err := ConvertToOpenAPI(fs, tt.crdContent, "test-crd.yaml", "base-folder")

			// Check if an error was expected
			if tt.expectedErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Perform validation for the success case (only needed if no error was expected)
			_, err = afero.Exists(fs, filepath.Join(outputPath, "template_fn_crossplane_io_v1beta1_kclinput.yaml"))
			require.NoError(t, err)

			// Read the content from the file in-memory
			output, err := afero.ReadFile(fs, outputPath)
			require.NoError(t, err)

			var openapi *spec3.OpenAPI
			err = yaml.Unmarshal(output, &openapi)
			require.NoError(t, err)

			apiVersionDefault := openapi.Components.Schemas["io.crossplane.fn.template.v1beta1.KCLInput"].SchemaProps.Properties["apiVersion"].Default
			require.Equal(t, "template.fn.crossplane.io/v1beta1", apiVersionDefault, "The default value of apiVersion does not match the expected content")

			kindDefault := openapi.Components.Schemas["io.crossplane.fn.template.v1beta1.KCLInput"].SchemaProps.Properties["kind"].Default
			require.Equal(t, "KCLInput", kindDefault, "The default value of kind does not match the expected content")
		})
	}
}

func TestAddDefaultAPIVersionAndKind(t *testing.T) {
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
			// Call the function to add default values
			addDefaultAPIVersionAndKind(&tt.initialSchema, tt.gvk)

			// Check the apiVersion default if the property exists
			if prop, ok := tt.initialSchema.Properties["apiVersion"]; ok {
				require.Equal(t, tt.expectedAPIVersion, prop.Default, "apiVersion default should match expected value")
			}

			// Check the kind default if the property exists
			if prop, ok := tt.initialSchema.Properties["kind"]; ok {
				require.Equal(t, tt.expectedKind, prop.Default, "kind default should match expected value")
			}
		})
	}
}
