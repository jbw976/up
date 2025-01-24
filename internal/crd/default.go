// Copyright 2025 Upbound Inc.
// All rights reserved

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
