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

// Package crd contains methods for working with CRDs.
package crd

import (
	"fmt"
	"strings"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	schema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	structuraldefaulting "k8s.io/apiextensions-apiserver/pkg/apiserver/schema/defaulting"
)

// DefaultValues sets default values on the XR based on the CRD schema.
func DefaultValues(xr map[string]any, crd extv1.CustomResourceDefinition) error {
	apiVersion, ok := xr["apiVersion"].(string)
	if !ok {
		return fmt.Errorf("apiVersion not found in xr")
	}

	// Extract the version from the apiVersion (format: "group/version")
	parts := strings.Split(apiVersion, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid apiVersion format in xr: %s", apiVersion)
	}
	xrVersion := parts[1]

	// Find the matching CRD version
	var schemaProps *extv1.JSONSchemaProps
	for _, v := range crd.Spec.Versions {
		if v.Name == xrVersion {
			if v.Schema != nil && v.Schema.OpenAPIV3Schema != nil {
				schemaProps = v.Schema.OpenAPIV3Schema
			}
			break
		}
	}

	if schemaProps == nil {
		return fmt.Errorf("matching CRD version not found for xr version: %s", xrVersion)
	}

	// Convert CRD schema to apiextensions format
	var k apiextensions.JSONSchemaProps
	err := extv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(schemaProps, &k, nil)
	if err != nil {
		return err
	}

	// Create structural schema with defaults
	crdWithDefaults, err := schema.NewStructural(&k)
	if err != nil {
		return err
	}

	// Apply structural defaults
	structuraldefaulting.Default(xr, crdWithDefaults)
	return nil
}
