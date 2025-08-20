// Copyright 2025 Upbound Inc.
// All rights reserved

package crd

import (
	"fmt"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
)

// InferProperties to return the correct type.
func InferProperties(spec map[string]interface{}) (map[string]extv1.JSONSchemaProps, error) {
	properties := make(map[string]extv1.JSONSchemaProps)

	for key, value := range spec {
		strKey := fmt.Sprintf("%v", key)
		inferredProp, err := inferProperty(value)
		if err != nil {
			// Return the error and propagate it upwards
			return nil, errors.Wrapf(err, "error inferring property for key '%s'", strKey)
		}
		properties[strKey] = inferredProp
	}

	return properties, nil
}

// inferArrayProperty handles array type inference with property merging for objects.
func inferArrayProperty(v []interface{}) (extv1.JSONSchemaProps, error) {
	if len(v) == 0 {
		// If the array is empty, default to array of objects
		return extv1.JSONSchemaProps{
			Type: "array",
			Items: &extv1.JSONSchemaPropsOrArray{
				Schema: &extv1.JSONSchemaProps{
					Type: "object",
				},
			},
		}, nil
	}

	// Infer the type of the first element
	firstElemSchema, err := inferProperty(v[0])
	if err != nil {
		return extv1.JSONSchemaProps{}, err
	}

	// Check if all elements are of the same type and merge object properties
	mergedProperties := make(map[string]extv1.JSONSchemaProps)
	if firstElemSchema.Type == "object" {
		// For objects, merge all properties from all elements
		for key, prop := range firstElemSchema.Properties {
			mergedProperties[key] = prop
		}
	}

	for _, elem := range v {
		elemSchema, err := inferProperty(elem)
		if err != nil {
			return extv1.JSONSchemaProps{}, err
		}
		if elemSchema.Type != firstElemSchema.Type {
			return extv1.JSONSchemaProps{}, errors.New("mixed types detected in array")
		}
		// If it's an object, merge additional properties
		if elemSchema.Type == "object" {
			for key, prop := range elemSchema.Properties {
				mergedProperties[key] = prop
			}
		}
	}

	// Build the result schema
	resultSchema := firstElemSchema
	if firstElemSchema.Type == "object" && len(mergedProperties) > 0 {
		resultSchema.Properties = mergedProperties
	}

	return extv1.JSONSchemaProps{
		Type: "array",
		Items: &extv1.JSONSchemaPropsOrArray{
			Schema: &resultSchema,
		},
	}, nil
}

// inferProperty to return extv1.JSONSchemaProps.
func inferProperty(value interface{}) (extv1.JSONSchemaProps, error) {
	// Explicitly handle nil
	if value == nil {
		return extv1.JSONSchemaProps{
			Type: "string", // Ensure this returns "string" for nil
		}, nil
	}

	switch v := value.(type) {
	case string:
		return extv1.JSONSchemaProps{
			Type: "string",
		}, nil
	case int, int32, int64:
		return extv1.JSONSchemaProps{
			Type: "integer",
		}, nil
	case float32, float64:
		return extv1.JSONSchemaProps{
			Type: "number",
		}, nil
	case bool:
		return extv1.JSONSchemaProps{
			Type: "boolean",
		}, nil
	case map[string]interface{}:
		// Recursively infer properties for nested objects and handle errors
		inferredProps, err := InferProperties(v)
		if err != nil {
			return extv1.JSONSchemaProps{}, errors.Wrap(err, "error inferring properties for object")
		}
		return extv1.JSONSchemaProps{
			Type:       "object",
			Properties: inferredProps,
		}, nil
	case []interface{}:
		return inferArrayProperty(v)
	default:
		// Return an error for unknown types (excluding nil which is handled earlier)
		return extv1.JSONSchemaProps{}, errors.Errorf("unknown type: %T", value)
	}
}
