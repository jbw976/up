// Copyright 2025 Upbound Inc.
// All rights reserved

package crd

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/afero"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/controller/openapi/builder"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kube-openapi/pkg/spec3"
	"k8s.io/kube-openapi/pkg/validation/spec"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// ToOpenAPI converts the storage version of a CRD to an OpenAPI spec. The
// version is returned along with the OpenAPI spec.
func ToOpenAPI(crd *extv1.CustomResourceDefinition) (map[string]*spec3.OpenAPI, error) {
	oapis := make(map[string]*spec3.OpenAPI, len(crd.Spec.Versions))

	if len(crd.Spec.Versions) == 0 {
		return nil, errors.New("crd has no versions")
	}

	for _, crdVersion := range crd.Spec.Versions {
		version := crdVersion.Name

		// Generate OpenAPI v3 schema
		output, err := builder.BuildOpenAPIV3(crd, version, builder.Options{})
		if err != nil {
			return nil, errors.Wrapf(err, "failed to build OpenAPI v3 schema")
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

		oapis[version] = output
	}

	return oapis, nil
}

// FilesToOpenAPI converts an on-disk CRD to an OpenAPI spec, and writes the
// OpenAPI spec to a file. The path to the spec is returned.
func FilesToOpenAPI(fs afero.Fs, bs []byte, path string) ([]string, error) {
	var crd extv1.CustomResourceDefinition
	if err := yaml.Unmarshal(bs, &crd); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal CRD file %q", path)
	}

	outputs, err := ToOpenAPI(&crd)
	if err != nil {
		return nil, err
	}

	paths := make([]string, 0, len(outputs))
	for version, output := range outputs {
		// Convert output to YAML
		openAPIBytes, err := yaml.Marshal(output)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to marshal OpenAPI output to YAML")
		}

		// Define the output path for the OpenAPI schema file
		groupFormatted := strings.ReplaceAll(crd.Spec.Group, ".", "_")
		kindFormatted := strings.ToLower(crd.Spec.Names.Kind)
		openAPIPath := fmt.Sprintf("%s_%s_%s.yaml", groupFormatted, version, kindFormatted)

		// Write the output to a file
		if err := afero.WriteFile(fs, openAPIPath, openAPIBytes, 0o644); err != nil {
			return nil, errors.Wrapf(err, "failed to write OpenAPI file")
		}

		paths = append(paths, openAPIPath)
	}

	return paths, nil
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
