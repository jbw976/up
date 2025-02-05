// Copyright 2025 Upbound Inc
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
			xr: map[string]any{},
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
										"field1": {Type: "string", Default: &apiextensionsv1.JSON{Raw: []byte(`"defaultValue"`)}},
									},
								},
							},
						},
					},
				},
			},
			want: map[string]any{
				"field1": "defaultValue",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := DefaultValues(tc.xr, tc.crd)
			if err != nil {
				t.Fatalf("DefaultValues() returned an error: %v", err)
			}

			if diff := cmp.Diff(tc.xr, tc.want); diff != "" {
				t.Errorf("DefaultValues() mismatch (-got +want):\n%s", diff)
			}
		})
	}
}
