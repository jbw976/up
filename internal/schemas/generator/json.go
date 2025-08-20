// Copyright 2025 Upbound Inc.
// All rights reserved

package generator

import (
	"context"
	"encoding/json"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/invopop/jsonschema"
	"github.com/spf13/afero"
	"k8s.io/kube-openapi/pkg/spec3"
	"k8s.io/kube-openapi/pkg/validation/spec"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/upbound/up/internal/schemas/runner"
)

type jsonGenerator struct{}

func (jsonGenerator) Language() string {
	return "json"
}

// GenerateFromCRD generates jsonschemas for the CRDs in the given filesystem. These
// can be used by editors when writing YAML, for example as part of Go
// templates.
func (jsonGenerator) GenerateFromCRD(_ context.Context, fromFS afero.Fs, _ runner.SchemaRunner) (afero.Fs, error) {
	openAPIs, err := goCollectOpenAPIs(fromFS)
	if err != nil {
		return nil, err
	}

	if len(openAPIs) == 0 {
		// Return nil if no specs were generated
		return nil, nil
	}

	schemaFS := afero.NewMemMapFs()
	if err := schemaFS.Mkdir("models", 0o755); err != nil {
		return nil, errors.Wrap(err, "failed to create models directory")
	}

	schemas := make(map[string]*spec.Schema)
	for _, oapi := range openAPIs {
		for name, s := range oapi.spec.Components.Schemas {
			schemas[name] = s
		}
	}

	for name, schema := range schemas {
		jschema, err := oapiSchemaToJSONSchema(schema)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to generate jsonschema for %s", name)
		}

		bs, err := json.Marshal(jschema)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to marshal jsonschema for %s", name)
		}

		// To keep references simple, we don't build a directory
		// hierarchy. Rather, we write a flat directory of files with
		// unambiguous names.
		//
		// E.g., the schema for kind MyDatabase in GV
		// platform.example.com/v1alpha1 goes in
		// com-example-platform-v1alpha1-MyDatabase.schema.json.
		fname := filepath.Join("models", strings.ReplaceAll(name, ".", "-")+".schema.json")
		if err := afero.WriteFile(schemaFS, fname, bs, 0o644); err != nil {
			return nil, errors.Wrapf(err, "failed to write jsonschema for %s", name)
		}
	}

	return schemaFS, nil
}

// oapiSchemaToJSONSchema converts a k8s OpenAPI schema to a JSON schema by
// round-tripping through JSON and mutating any references to work with our file
// naming scheme.
func oapiSchemaToJSONSchema(s *spec.Schema) (*jsonschema.Schema, error) {
	bs, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}

	var conv jsonschema.Schema
	if err := json.Unmarshal(bs, &conv); err != nil {
		return nil, err
	}

	return mutateJSONSchema(&conv), nil
}

// mutateJSONSchema recursively replaces any internal references in a converted
// JSON schema with references to other schema files we're generating. It also
// sets the additionalProperties field to false on objects that don't already
// have it set otherwise so that our schemas disallow extra fields.
func mutateJSONSchema(s *jsonschema.Schema) *jsonschema.Schema {
	if s.Type == "object" && s.AdditionalProperties == nil {
		// Disallow additional properties so the yaml-language-server will throw
		// an error on invalid field names.
		s.AdditionalProperties = jsonschema.FalseSchema
	}

	if strings.HasPrefix(s.Ref, "#/components/schemas/") {
		s.Ref = strings.TrimPrefix(s.Ref, "#/components/schemas/")
		s.Ref = strings.ReplaceAll(s.Ref, ".", "-")
		s.Ref += ".schema.json"
	}

	for i, schema := range s.AllOf {
		rep := mutateJSONSchema(schema)
		s.AllOf[i] = rep
	}
	for i, schema := range s.AnyOf {
		rep := mutateJSONSchema(schema)
		s.AnyOf[i] = rep
	}
	for i, schema := range s.OneOf {
		rep := mutateJSONSchema(schema)
		s.OneOf[i] = rep
	}
	if s.Not != nil {
		s.Not = mutateJSONSchema(s.Not)
	}

	if s.Items != nil {
		s.Items = mutateJSONSchema(s.Items)
	}

	if s.AdditionalProperties != nil {
		s.AdditionalProperties = mutateJSONSchema(s.AdditionalProperties)
	}

	for prop := s.Properties.Oldest(); prop != nil; prop = prop.Next() {
		rep := mutateJSONSchema(prop.Value)
		s.Properties.Set(prop.Key, rep)
	}

	return s
}

// GenerateFromOpenAPI generates jsonschemas from OpenAPI v3 specs in the given filesystem.
func (jsonGenerator) GenerateFromOpenAPI(_ context.Context, fromFS afero.Fs, _ runner.SchemaRunner) (afero.Fs, error) {
	// Collect all OpenAPI v3 JSON files
	var openAPISpecs []*spec3.OpenAPI
	err := afero.Walk(fromFS, "", func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Only process .json files
		if filepath.Ext(path) != ".json" {
			return nil
		}

		// Read the file
		bs, err := afero.ReadFile(fromFS, path)
		if err != nil {
			return errors.Wrapf(err, "failed to read OpenAPI file %q", path)
		}

		// Parse as OpenAPI v3 spec
		var openAPI spec3.OpenAPI
		if err := json.Unmarshal(bs, &openAPI); err != nil {
			// Skip files that aren't valid OpenAPI specs
			return nil //nolint:nilerr // See comment above.
		}

		// Only process if it has components/schemas
		if openAPI.Components != nil && len(openAPI.Components.Schemas) > 0 {
			openAPISpecs = append(openAPISpecs, &openAPI)
		}

		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to walk OpenAPI filesystem")
	}

	if len(openAPISpecs) == 0 {
		// Return nil if no specs were found
		return nil, nil
	}

	// Create output filesystem
	schemaFS := afero.NewMemMapFs()
	if err := schemaFS.Mkdir("models", 0o755); err != nil {
		return nil, errors.Wrap(err, "failed to create models directory")
	}

	// Collect all schemas from all OpenAPI specs
	schemas := make(map[string]*spec.Schema)
	for _, oapi := range openAPISpecs {
		for name, s := range oapi.Components.Schemas {
			schemas[name] = s
		}
	}

	// Generate JSON schemas
	for name, schema := range schemas {
		jschema, err := oapiSchemaToJSONSchema(schema)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to generate jsonschema for %s", name)
		}

		bs, err := json.Marshal(jschema)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to marshal jsonschema for %s", name)
		}

		// To keep references simple, we don't build a directory
		// hierarchy. Rather, we write a flat directory of files with
		// unambiguous names.
		fname := filepath.Join("models", strings.ReplaceAll(name, ".", "-")+".schema.json")
		if err := afero.WriteFile(schemaFS, fname, bs, 0o644); err != nil {
			return nil, errors.Wrapf(err, "failed to write jsonschema for %s", name)
		}
	}

	return schemaFS, nil
}
