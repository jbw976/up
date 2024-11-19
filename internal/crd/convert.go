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
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/afero"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/controller/openapi/builder"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kube-openapi/pkg/validation/spec"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

func ConvertToOpenAPI(fs afero.Fs, bs []byte, path, baseFolder string) (string, error) {
	var crd extv1.CustomResourceDefinition
	if err := yaml.Unmarshal(bs, &crd); err != nil {
		return "", errors.Wrapf(err, "failed to unmarshal CRD file %q", path)
	}

	version, err := GetCRDVersion(crd)
	if err != nil {
		return "", err
	}

	// Generate OpenAPI v3 schema
	output, err := builder.BuildOpenAPIV3(&crd, version, builder.Options{})
	if err != nil {
		return "", errors.Wrapf(err, "failed to build OpenAPI v3 schema")
	}

	// Reverse the group name inline
	groupParts := strings.Split(crd.Spec.Group, ".")
	slices.Reverse(groupParts)
	reverseGroup := strings.Join(groupParts, ".")

	// Process schemas to include API version and kind for matching CR versions
	for name, s := range output.Components.Schemas {
		if !strings.HasPrefix(name, reverseGroup+".") {
			continue // Skip schemas not in our API group
		}

		if fmt.Sprintf("%s.%s.%s", reverseGroup, version, crd.Spec.Names.Kind) == name {
			addDefaultAPIVersionAndKind(s, schema.GroupVersionKind{
				Group:   crd.Spec.Group,
				Version: version,
				Kind:    crd.Spec.Names.Kind,
			})
		}
	}

	// Convert output to YAML
	openAPIBytes, err := yaml.Marshal(output)
	if err != nil {
		return "", errors.Wrapf(err, "failed to marshal OpenAPI output to YAML")
	}

	// Define the output path for the OpenAPI schema file
	groupFormatted := strings.ReplaceAll(crd.Spec.Group, ".", "_")
	kindFormatted := strings.ToLower(crd.Spec.Names.Kind)
	openAPIPath := fmt.Sprintf("%s_%s_%s.yaml", groupFormatted, version, kindFormatted)

	// Write the output to a file
	if err := afero.WriteFile(fs, openAPIPath, openAPIBytes, 0o644); err != nil {
		return "", errors.Wrapf(err, "failed to write OpenAPI file")
	}

	return openAPIPath, nil
}

func addDefaultAPIVersionAndKind(s *spec.Schema, gvk schema.GroupVersionKind) {
	if prop, ok := s.Properties["apiVersion"]; ok {
		prop.Default = gvk.GroupVersion().String()
		s.Properties["apiVersion"] = prop
	}
	if prop, ok := s.Properties["kind"]; ok {
		prop.Default = gvk.Kind
		s.Properties["kind"] = prop
	}
}
