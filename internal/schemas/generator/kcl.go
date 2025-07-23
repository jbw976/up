// Copyright 2025 Upbound Inc.
// All rights reserved

// Package generator generates language-specific schemas for Crossplane and
// Kubernetes resources.
package generator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/spf13/afero"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kube-openapi/pkg/spec3"
	"k8s.io/kube-openapi/pkg/validation/spec"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	xpv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"

	xcrd "github.com/upbound/up/internal/crd"
	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/internal/schemas/runner"
	"github.com/upbound/up/internal/version"
)

const (
	kclSchemaFolder         = "schemas"
	kclModelsFolder         = "models"
	kclAdoptModelsStructure = "sorted"
	kclImage                = "xpkg.upbound.io/upbound/kcl:v0.11.2"
)

type kclGenerator struct{}

func (kclGenerator) Language() string {
	return "kcl"
}

// GenerateFromCRD generates KCL schema files from the XRDs and CRDs fromFS.
func (kclGenerator) GenerateFromCRD(ctx context.Context, fromFS afero.Fs, generator runner.SchemaRunner) (afero.Fs, error) { //nolint:gocognit // generate kcl schemas
	crdFS := afero.NewMemMapFs()
	schemaFS := afero.NewMemMapFs()
	baseFolder := "workdir"

	if err := crdFS.MkdirAll(baseFolder, 0o755); err != nil {
		return nil, err
	}

	var crdPaths []string

	// Walk the virtual filesystem to find and process target files
	if err := afero.Walk(fromFS, "", func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}
		// Ignore files without yaml extensions.
		ext := filepath.Ext(path)
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		var u metav1.TypeMeta
		bs, err := afero.ReadFile(fromFS, path)
		if err != nil {
			return errors.Wrapf(err, "failed to read file %q", path)
		}
		err = yaml.Unmarshal(bs, &u)
		if err != nil {
			return errors.Wrapf(err, "failed to parse file %q", path)
		}

		switch u.GroupVersionKind().Kind {
		case xpv1.CompositeResourceDefinitionKind:
			// Process the XRD and get the paths
			xrPath, claimPath, err := xcrd.ProcessXRD(crdFS, bs, path, baseFolder)
			if err != nil {
				return err
			}

			// Append paths if they are returned
			if xrPath != "" {
				crdPaths = append(crdPaths, xrPath)
			}
			if claimPath != "" {
				crdPaths = append(crdPaths, claimPath)
			}

		case "CustomResourceDefinition":
			// Write CRD file
			if err := afero.WriteFile(crdFS, filepath.Join(baseFolder, path), bs, 0o644); err != nil {
				return err
			}
			crdPaths = append(crdPaths, filepath.Join(baseFolder, path))
		}

		return nil
	}); err != nil {
		return nil, err
	}

	if len(crdPaths) == 0 {
		// Return nil if no files were generated
		return nil, nil
	}

	if err := generator.Generate(
		ctx,
		crdFS,
		baseFolder,
		"",
		kclImage,
		[]string{
			"sh", "-c",
			`find . -name "*.yaml" -exec kcl import -m crd -s {} \;`,
		},
	); err != nil {
		return nil, err
	}

	// we need to transform the folder structure to avoid the same resource kinds across multiple providers
	if err := transformStructureKcl(crdFS, kclModelsFolder, kclAdoptModelsStructure); err != nil {
		return nil, err
	}

	// Copy only the files from kclAdoptModelsStructure into the schemaFs
	if err := filesystem.CopyFilesBetweenFs(afero.NewBasePathFs(crdFS, kclAdoptModelsStructure), afero.NewBasePathFs(schemaFS, kclModelsFolder)); err != nil {
		return nil, err
	}

	return schemaFS, nil
}

// transformStructureKcl modifies the existing fs by moving files from sourceDir to targetDir
// in the reversed and segmented structure with the version appended at the end,
// and it copies the k8s directory and specific files (kcl.mod and kcl.mod.lock) to the targetDir.
func transformStructureKcl(fs afero.Fs, sourceDir, targetDir string) error { //nolint:gocognit // tansform kcl schemas
	// Copy kcl.mod and kcl.mod.lock files if they exist
	if err := filesystem.CopyFileIfExists(fs, filepath.Join(sourceDir, "kcl.mod"), filepath.Join(targetDir, "kcl.mod")); err != nil {
		return errors.Wrap(err, "failed to copy kcl.mod")
	}

	if err := filesystem.CopyFileIfExists(fs, filepath.Join(sourceDir, "kcl.mod.lock"), filepath.Join(targetDir, "kcl.mod.lock")); err != nil {
		return errors.Wrap(err, "failed to copy kcl.mod.lock")
	}

	// Modify files in the sourceDir before copying
	objectMetaPath := filepath.Join(sourceDir, "k8s", "apimachinery", "pkg", "apis", "meta", "v1", "object_meta.k")
	managedFieldsEntryPath := filepath.Join(sourceDir, "k8s", "apimachinery", "pkg", "apis", "meta", "v1", "managed_fields_entry.k")

	// Replace `managedFields?: [ManagedFieldsEntry]` with `managedFields?: any` in `object_meta.k`
	if _, err := fs.Stat(objectMetaPath); err == nil { // Check if the file exists
		content, err := afero.ReadFile(fs, objectMetaPath)
		if err != nil {
			return errors.Wrapf(err, "failed to read %s", objectMetaPath)
		}

		updatedContent := strings.ReplaceAll(string(content), "managedFields?: [ManagedFieldsEntry]", "managedFields?: any")

		if err := afero.WriteFile(fs, objectMetaPath, []byte(updatedContent), 0o644); err != nil {
			return errors.Wrapf(err, "failed to update %s", objectMetaPath)
		}
	}

	// Remove `managed_fields_entry.k` if it exists
	if _, err := fs.Stat(managedFieldsEntryPath); err == nil {
		if err := fs.Remove(managedFieldsEntryPath); err != nil {
			return errors.Wrapf(err, "failed to remove %s", managedFieldsEntryPath)
		}
	}

	// Copy the modified `k8s` directory to the targetDir
	k8sSourcePath := filepath.Join(sourceDir, "k8s")
	if err := filesystem.CopyFolder(fs, k8sSourcePath, filepath.Join(targetDir, "k8s")); err != nil {
		return errors.Wrap(err, "failed to copy k8s directory")
	}

	// Additional transformations remain the same, working on other files
	if err := afero.Walk(fs, sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || strings.HasPrefix(path, filepath.Join(sourceDir, "k8s")) {
			return nil
		}

		filename := info.Name()
		parts := strings.Split(filename, "_")

		// Identify the index of the known API version in the filename
		var versionIndex int
		foundVersion := false

		for i, part := range parts {
			if xcrd.IsKnownAPIVersion(part) {
				versionIndex = i
				foundVersion = true
				break
			}
		}

		if !foundVersion || versionIndex == 0 {
			return nil
		}

		// Take the segments before the version, reverse them, and append the version
		reversedParts := parts[:versionIndex]
		slices.Reverse(reversedParts)
		reversedParts = append(reversedParts, parts[versionIndex])

		// Construct the new directory structure by joining reversed parts
		newDir := filepath.Join(targetDir, filepath.Join(reversedParts...))

		// Ensure the new directory structure exists
		if err := fs.MkdirAll(newDir, 0o755); err != nil {
			return errors.Wrapf(err, "failed to create directory %s", newDir)
		}

		// Transform the filename after the version: remove underscores
		transformedName := strings.ReplaceAll(strings.Join(parts[versionIndex+1:], ""), "_", "")
		transformedName = strings.ReplaceAll(transformedName, "swagger", "")

		// Construct the new file path in the target directory with the transformed name
		newFilePath := filepath.Join(newDir, transformedName)

		// Copy the file to the new location
		srcFile, err := fs.Open(path)
		if err != nil {
			return errors.Wrapf(err, "failed to open source file %s", path)
		}

		destFile, err := fs.Create(newFilePath)
		if err != nil {
			return errors.Wrapf(err, "failed to create destination file %s", newFilePath)
		}

		_, err = io.Copy(destFile, srcFile)
		if err != nil {
			return errors.Wrapf(err, "failed to copy file from %s to %s", path, newFilePath)
		}

		return nil
	}); err != nil {
		return errors.Wrap(err, "error processing directory")
	}

	return nil
}

// GenerateFromOpenAPI generates KCL schema files from OpenAPI v3 specifications in fromFS.
func (kclGenerator) GenerateFromOpenAPI(_ context.Context, fromFS afero.Fs, _ runner.SchemaRunner) (afero.Fs, error) {
	schemaFS := afero.NewMemMapFs()

	// Create models directory
	if err := schemaFS.MkdirAll(kclModelsFolder, 0o755); err != nil {
		return nil, errors.Wrap(err, "failed to create models directory")
	}

	// Collect all OpenAPI v3 specs from JSON files
	openAPISpecs := make(map[string]*spec3.OpenAPI)

	// Walk the filesystem to find JSON files
	if err := afero.Walk(fromFS, "", func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Only process JSON files
		if !strings.HasSuffix(strings.ToLower(path), ".json") {
			return nil
		}

		// Read the file content
		bs, err := afero.ReadFile(fromFS, path)
		if err != nil {
			return errors.Wrapf(err, "failed to read file %q", path)
		}

		// Parse as OpenAPI v3 spec
		var openAPI spec3.OpenAPI
		if err := json.Unmarshal(bs, &openAPI); err != nil {
			// Skip files that aren't valid OpenAPI specs
			return nil //nolint:nilerr // See comment above.
		}

		// Only process if it has components/schemas
		if openAPI.Components != nil && len(openAPI.Components.Schemas) > 0 {
			openAPISpecs[path] = &openAPI
		}

		return nil
	}); err != nil {
		return nil, errors.Wrap(err, "failed to walk OpenAPI filesystem")
	}

	if len(openAPISpecs) == 0 {
		// Return nil if no specs were found
		return nil, nil
	}

	// Process all schemas from all OpenAPI specs and add defaults
	allSchemas := make(map[string]*spec.Schema)
	for _, oapi := range openAPISpecs {
		// Add defaults for apiVersion and kind based on x-kubernetes-group-version-kind
		addKCLDefaults(oapi)

		if oapi.Components != nil {
			for name, s := range oapi.Components.Schemas {
				allSchemas[name] = s
			}
		}
	}

	generatedSchemas := make(map[string]string)
	for name, schema := range allSchemas {
		kclContent := generateKCLFile(name, schema, allSchemas)
		generatedSchemas[name] = kclContent

		filename := filepath.Join(kclModelsFolder, toKCLFileName(name))

		dir := filepath.Dir(filename)
		if err := schemaFS.MkdirAll(dir, 0o755); err != nil {
			return nil, errors.Wrapf(err, "failed to create directory for %s", name)
		}

		if err := afero.WriteFile(schemaFS, filename, []byte(kclContent), 0o644); err != nil {
			return nil, errors.Wrapf(err, "failed to write KCL schema for %s", name)
		}
	}

	kclModContent := `[package]
name = "models"
edition = "v0.10.0"
version = "0.0.1"
`
	if err := afero.WriteFile(schemaFS, filepath.Join(kclModelsFolder, "kcl.mod"), []byte(kclModContent), 0o644); err != nil {
		return nil, errors.Wrap(err, "failed to write kcl.mod")
	}

	return schemaFS, nil
}

// toKCLFileName converts a schema name to a KCL filename with proper folder structure.
func toKCLFileName(name string) string {
	// Split by dots to create folder hierarchy
	parts := strings.Split(name, ".")
	if len(parts) == 0 {
		return name + ".k"
	}

	// The last part is the filename, everything else is the path
	if len(parts) == 1 {
		return parts[0] + ".k"
	}

	// Create path from all parts except the last one
	path := filepath.Join(parts[:len(parts)-1]...)
	filename := parts[len(parts)-1] + ".k"

	return filepath.Join(path, filename)
}

// extractSchemaName extracts schema name from a reference.
func extractSchemaName(ref string) string {
	if ref == "" {
		return ""
	}
	// Handle references like #/components/schemas/SchemaName
	parts := strings.Split(ref, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

// extractSimpleName extracts the simple name from a full schema name.
// e.g. "io.k8s.api.apps.v1.Deployment" -> "Deployment".
func extractSimpleName(fullName string) string {
	parts := strings.Split(fullName, ".")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return fullName
}

// handleAllOfType processes allOf schemas and returns the KCL type if found.
func handleAllOfType(schema *spec.Schema, currentSchemaName string) (string, bool) {
	if len(schema.AllOf) == 0 {
		return "", false
	}

	for _, allOfSchema := range schema.AllOf {
		if allOfSchema.Ref.String() != "" {
			if kclType := processSchemaReference(allOfSchema.Ref.String(), currentSchemaName); kclType != "" {
				return kclType, true
			}
		}
	}
	return "", false
}

// processSchemaReference converts a schema reference to KCL type.
func processSchemaReference(ref string, currentSchemaName string) string {
	refName := extractSchemaName(ref)
	if refName == "" {
		return ""
	}

	// Special case for IntOrString - convert to union type
	if strings.HasSuffix(refName, "IntOrString") {
		return "int | str"
	}
	// Special case for Quantity - convert to string type
	if strings.HasSuffix(refName, "Quantity") {
		return "str"
	}
	// Special case for Time - convert to string type
	if strings.HasSuffix(refName, "Time") {
		return "str"
	}
	// Special case for RawExtension - convert to any
	if strings.HasSuffix(refName, "RawExtension") {
		return "any"
	}
	return formatTypeReference(refName, currentSchemaName)
}

// handleArrayType processes array schemas and returns the KCL type.
func handleArrayType(schema *spec.Schema, allSchemas map[string]*spec.Schema, currentSchemaName string) (string, bool) {
	if !schema.Type.Contains("array") || schema.Items == nil || schema.Items.Schema == nil {
		return "", false
	}

	itemType := convertOpenAPITypeToKCL(schema.Items.Schema, allSchemas, currentSchemaName)
	return "[" + itemType + "]", true
}

// handleObjectType processes object schemas and returns the KCL type.
func handleObjectType(schema *spec.Schema, allSchemas map[string]*spec.Schema, currentSchemaName string) (string, bool) {
	if !schema.Type.Contains("object") {
		return "", false
	}

	// Check if this has additionalProperties which makes it a map
	if schema.AdditionalProperties != nil && schema.AdditionalProperties.Schema != nil {
		valueType := convertOpenAPITypeToKCL(schema.AdditionalProperties.Schema, allSchemas, currentSchemaName)
		return "{str:" + valueType + "}", true
	}
	if len(schema.Properties) == 0 {
		return "{str: any}", true
	}
	// For objects with properties, we'll generate a schema
	return "dict", true
}

// convertOpenAPITypeToKCL converts OpenAPI types to KCL types about the current schema.
func convertOpenAPITypeToKCL(schema *spec.Schema, allSchemas map[string]*spec.Schema, currentSchemaName string) string {
	if schema == nil {
		return "any"
	}

	// Handle allOf
	if kclType, found := handleAllOfType(schema, currentSchemaName); found {
		return kclType
	}

	// Handle direct references
	if schema.Ref.String() != "" {
		if kclType := processSchemaReference(schema.Ref.String(), currentSchemaName); kclType != "" {
			return kclType
		}
	}

	// Handle arrays
	if kclType, found := handleArrayType(schema, allSchemas, currentSchemaName); found {
		return kclType
	}

	// Handle objects
	if kclType, found := handleObjectType(schema, allSchemas, currentSchemaName); found {
		return kclType
	}

	// Handle basic types
	switch {
	case schema.Type.Contains("string"):
		return "str"
	case schema.Type.Contains("integer"):
		return "int"
	case schema.Type.Contains("number"):
		return "float"
	case schema.Type.Contains("boolean"):
		return "bool"
	default:
		return "any"
	}
}

// formatTypeReference formats a type reference based on its package and the current context.
func formatTypeReference(refName, currentSchemaName string) string {
	if strings.HasPrefix(refName, "io.k8s.api.") {
		lastDot := strings.LastIndex(refName, ".")
		if lastDot > 0 {
			// Check we have at least io.k8s.api.<group>.<version>.<Type> format
			packagePath := refName[:lastDot]
			if strings.Count(packagePath, ".") >= 4 { // At least 4 dots before the type name
				typeName := refName[lastDot+1:]

				// Check if current schema is in the same package
				if strings.HasPrefix(currentSchemaName, packagePath+".") {
					// Same package, no prefix needed
					return typeName
				}

				// Different package, need import alias
				alias := getImportAlias(packagePath)
				return alias + "." + typeName
			}
		}
	}

	// Check if this is from the meta.v1 package
	if typeName, ok := strings.CutPrefix(refName, "io.k8s.apimachinery.pkg.apis.meta.v1."); ok {
		// If current schema is also in meta.v1, just use the type name without prefix
		if strings.HasPrefix(currentSchemaName, "io.k8s.apimachinery.pkg.apis.meta.v1.") {
			return typeName
		}
		// Otherwise, use v1. prefix
		return "v1." + typeName
	}

	// Check if this is from the runtime package
	if typeName, ok := strings.CutPrefix(refName, "io.k8s.apimachinery.pkg.runtime."); ok {
		// If current schema is also in runtime package, just use the type name without prefix
		if strings.HasPrefix(currentSchemaName, "io.k8s.apimachinery.pkg.runtime.") {
			return typeName
		}
		// Otherwise, use runtime. prefix
		return "runtime." + typeName
	}

	// Check if this is from any other apimachinery package
	if strings.HasPrefix(refName, "io.k8s.apimachinery.pkg.") {
		parts := strings.Split(refName, ".")
		if len(parts) > 1 {
			// Get the package path without the type name
			packagePath := strings.Join(parts[:len(parts)-1], ".")
			typeName := parts[len(parts)-1]

			// Check if current schema is in the same package
			if strings.HasPrefix(currentSchemaName, packagePath+".") {
				// Same package, no prefix needed
				return typeName
			}

			// Different package, need import alias
			alias := getImportAlias(packagePath)
			return alias + "." + typeName
		}
	}

	// For other types, just use the simple name
	return extractSimpleName(refName)
}

// getImportAlias generates an import alias for a package path.
func getImportAlias(packagePath string) string {
	// Special case for meta.v1
	if packagePath == "io.k8s.apimachinery.pkg.apis.meta.v1" {
		return "v1"
	}

	// Special case for runtime
	if packagePath == "io.k8s.apimachinery.pkg.runtime" {
		return "runtime"
	}

	// For io.k8s.api packages, create alias from group and version
	if strings.HasPrefix(packagePath, "io.k8s.api.") {
		parts := strings.Split(packagePath, ".")
		if len(parts) >= 5 { // io.k8s.api.<group>.<version>
			group := parts[3]
			version := parts[4]
			// Create alias like "coreV1", "appsV1", etc.
			// Use proper title casing for the version part
			caser := cases.Title(language.English)
			versionTitle := caser.String(version)
			return group + versionTitle
		}
	}

	// For other apimachinery packages
	if strings.HasPrefix(packagePath, "io.k8s.apimachinery.pkg.") {
		parts := strings.Split(packagePath, ".")
		// Extract the last part(s) for the alias
		if len(parts) >= 5 {
			// io.k8s.apimachinery.pkg.<something>
			if parts[4] == "apis" && len(parts) >= 6 {
				// io.k8s.apimachinery.pkg.apis.<group>.<version>
				group := parts[len(parts)-2]
				version := parts[len(parts)-1]
				// Use proper title casing for camelCase style
				caser := cases.Title(language.English)
				versionTitle := caser.String(version)
				return group + versionTitle
			}
			// io.k8s.apimachinery.pkg.<package>
			return parts[4]
		}
	}

	// Default: use last two parts in camelCase
	parts := strings.Split(packagePath, ".")
	if len(parts) >= 2 {
		lastPart := parts[len(parts)-1]
		secondLastPart := parts[len(parts)-2]
		// Use proper title casing for camelCase style
		caser := cases.Title(language.English)
		lastPartTitle := caser.String(lastPart)
		return secondLastPart + lastPartTitle
	}
	return "unknown"
}

// processSchemaProperties extracts and merges properties from schema and allOf schemas.
func processSchemaProperties(schema *spec.Schema) (map[string]*spec.Schema, map[string]bool, []string) {
	properties := make(map[string]*spec.Schema)
	requiredSet := make(map[string]bool)

	// Collect required fields
	for _, req := range schema.Required {
		requiredSet[req] = true
	}

	// Merge properties from allOf
	if len(schema.AllOf) > 0 {
		for _, allOfSchema := range schema.AllOf {
			if allOfSchema.Properties != nil {
				for k, v := range allOfSchema.Properties {
					propCopy := v
					properties[k] = &propCopy
				}
			}
			// Merge required fields
			for _, req := range allOfSchema.Required {
				requiredSet[req] = true
			}
		}
	}

	// Add direct properties
	if schema.Properties != nil {
		for k, v := range schema.Properties {
			propCopy := v
			properties[k] = &propCopy
		}
	}

	// Sort property names for consistent output
	propNames := make([]string, 0, len(properties))
	for name := range properties {
		propNames = append(propNames, name)
	}
	sort.Strings(propNames)

	return properties, requiredSet, propNames
}

// generateDocStringHeader generates the docstring header with description.
func generateDocStringHeader(sb *strings.Builder, schema *spec.Schema) {
	if schema.Description != "" || len(schema.Properties) > 0 {
		sb.WriteString("    \"\"\"\n")

		// Add main description
		if schema.Description != "" {
			lines := strings.Split(strings.TrimSpace(schema.Description), "\n")
			for _, line := range lines {
				sb.WriteString("    " + strings.TrimSpace(line) + "\n")
			}
		}
	}
}

// generateAttributesDocumentation generates the attributes section in the docstring.
func generateAttributesDocumentation(sb *strings.Builder, propNames []string, properties map[string]*spec.Schema, requiredSet map[string]bool, allSchemas map[string]*spec.Schema, currentSchemaName string) {
	if len(propNames) == 0 {
		return
	}

	sb.WriteString("\n    Attributes\n")
	sb.WriteString("    ----------\n")

	for _, propName := range propNames {
		prop := properties[propName]

		// Format property name for documentation (handle special names)
		docPropName := propName
		if propName == "type" {
			docPropName = "$type"
		}

		// Build property documentation line
		sb.WriteString("    " + docPropName + " : ")
		sb.WriteString(convertOpenAPITypeToKCL(prop, allSchemas, currentSchemaName))

		// Add default value in docs
		if prop.Default != nil {
			sb.WriteString(", default is ")
			sb.WriteString(formatDefaultValue(prop.Default))
		} else {
			sb.WriteString(", default is Undefined")
		}

		// Add required/optional
		if requiredSet[propName] {
			sb.WriteString(", required")
		} else {
			sb.WriteString(", optional")
		}
		sb.WriteString("\n")

		// Add property description
		if prop.Description != "" {
			lines := strings.Split(strings.TrimSpace(prop.Description), "\n")
			for _, line := range lines {
				sb.WriteString("        " + strings.TrimSpace(line) + "\n")
			}
		}
	}
}

// generatePropertyField generates a single property field declaration.
func generatePropertyField(sb *strings.Builder, propName string, prop *spec.Schema, requiredSet map[string]bool, allSchemas map[string]*spec.Schema, currentSchemaName string) {
	// Handle special field names
	fieldName := propName
	if propName == "type" {
		fieldName = "$type"
	}

	// Add newline before property
	sb.WriteString("\n")

	// Property declaration
	sb.WriteString("    " + fieldName)

	// Add optional marker if not required
	if !requiredSet[propName] {
		sb.WriteString("?")
	}

	sb.WriteString(": ")
	propType := convertOpenAPITypeToKCL(prop, allSchemas, currentSchemaName)

	// Always write the property type first
	sb.WriteString(propType)

	// Add default value
	if prop.Default != nil {
		sb.WriteString(" = ")
		sb.WriteString(formatDefaultValue(prop.Default))
	}
}

// generateKCLSchema generates a KCL schema from an OpenAPI schema.
func generateKCLSchema(name string, schema *spec.Schema, allSchemas map[string]*spec.Schema, currentSchemaName string) string {
	var sb strings.Builder

	// Schema declaration
	sb.WriteString("schema " + name + ":\n")

	// Generate docstring header
	generateDocStringHeader(&sb, schema)

	// Process properties
	properties, requiredSet, propNames := processSchemaProperties(schema)

	// Generate attributes documentation
	generateAttributesDocumentation(&sb, propNames, properties, requiredSet, allSchemas, currentSchemaName)

	// Close docstring
	if schema.Description != "" || len(schema.Properties) > 0 {
		sb.WriteString("    \"\"\"\n\n")
	}

	// Generate property fields
	for _, propName := range propNames {
		prop := properties[propName]
		generatePropertyField(&sb, propName, prop, requiredSet, allSchemas, currentSchemaName)
	}

	return sb.String()
}

// formatDefaultValue formats a default value for KCL.
func formatDefaultValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return fmt.Sprintf("%q", v)
	case bool:
		if v {
			return "True"
		}
		return "False"
	case nil:
		return "None"
	case map[string]interface{}:
		if len(v) == 0 {
			return "{}"
		}
		return fmt.Sprintf("%v", v)
	case []interface{}:
		if len(v) == 0 {
			return "[]"
		}
		return fmt.Sprintf("%v", v)
	default:
		// Special handling for empty maps/arrays in Go
		str := fmt.Sprintf("%v", v)
		if str == "map[]" {
			return "{}"
		}
		if str == "[]" {
			return "[]"
		}
		return str
	}
}

// generateKCLFile generates a complete KCL file.
func generateKCLFile(fullSchemaName string, schema *spec.Schema, allSchemas map[string]*spec.Schema) string {
	// Extract simple name from full name
	name := extractSimpleName(fullSchemaName)
	var sb strings.Builder

	// Add file header
	sb.WriteString(fmt.Sprintf(`"""
This file was generated by up-cli version %s. DO NOT EDIT.
"""
`, version.Version()))

	// Collect all imports needed
	imports := make(map[string]bool)
	visited := make(map[*spec.Schema]bool)

	// Check properties for references
	if schema.Properties != nil {
		for _, prop := range schema.Properties {
			checkForImports(&prop, imports, visited)
		}
	}

	// Check allOf for references
	for _, allOfSchema := range schema.AllOf {
		if allOfSchema.Ref.String() != "" {
			refName := extractSchemaName(allOfSchema.Ref.String())
			addImportIfNeeded(refName, imports)
		}
	}

	// Remove imports for the same package as the current schema
	if fullSchemaName != "" {
		// Get the package path of the current schema
		lastDot := strings.LastIndex(fullSchemaName, ".")
		if lastDot > 0 {
			currentPackage := fullSchemaName[:lastDot]
			// Remove self-import
			delete(imports, currentPackage)
		}
	}

	// Add imports if needed
	for imp := range imports {
		alias := getImportAlias(imp)
		sb.WriteString("import " + imp + " as " + alias + "\n")
	}
	if len(imports) > 0 {
		sb.WriteString("\n")
	}

	// Generate the schema
	sb.WriteString(generateKCLSchema(name, schema, allSchemas, fullSchemaName))

	return sb.String()
}

// checkForImports checks a schema for references that need imports.
func checkForImports(schema *spec.Schema, imports map[string]bool, visited map[*spec.Schema]bool) {
	if schema == nil {
		return
	}

	// Skip if already visited to prevent cycles
	if visited[schema] {
		return
	}
	visited[schema] = true

	// Check allOf references (for properties like metadata, spec that use allOf)
	if len(schema.AllOf) > 0 {
		for _, allOfSchema := range schema.AllOf {
			if allOfSchema.Ref.String() != "" {
				refName := extractSchemaName(allOfSchema.Ref.String())
				// Skip imports for types that are converted to native types
				if !strings.HasSuffix(refName, "IntOrString") && !strings.HasSuffix(refName, "RawExtension") && !strings.HasSuffix(refName, "Quantity") && !strings.HasSuffix(refName, "Time") {
					addImportIfNeeded(refName, imports)
				}
				// Don't recurse into references - we only need the import
			}
		}
	}

	// Check direct references
	if schema.Ref.String() != "" {
		refName := extractSchemaName(schema.Ref.String())
		// Skip imports for types that are converted to native types
		if !strings.HasSuffix(refName, "IntOrString") && !strings.HasSuffix(refName, "RawExtension") && !strings.HasSuffix(refName, "Quantity") && !strings.HasSuffix(refName, "Time") {
			addImportIfNeeded(refName, imports)
		}
		// Don't recurse into references - we only need the import
		return
	}

	// Only check nested schemas if this is not a reference
	// Check array items
	if schema.Items != nil && schema.Items.Schema != nil {
		checkForImports(schema.Items.Schema, imports, visited)
	}

	// Check properties
	if schema.Properties != nil {
		for _, prop := range schema.Properties {
			checkForImports(&prop, imports, visited)
		}
	}

	// Check additionalProperties
	if schema.AdditionalProperties != nil && schema.AdditionalProperties.Schema != nil {
		checkForImports(schema.AdditionalProperties.Schema, imports, visited)
	}
}

// addImportIfNeeded adds an import for a reference if it's an external k8s type.
func addImportIfNeeded(refName string, imports map[string]bool) {
	if refName == "" {
		return
	}

	// Check if this is any k8s type (io.k8s.*)
	if strings.HasPrefix(refName, "io.k8s.") {
		// Extract package path (everything except the type name)
		lastDot := strings.LastIndex(refName, ".")
		if lastDot > 0 {
			packagePath := refName[:lastDot]
			imports[packagePath] = true
		}
	}
}

// addKCLDefaults adds default values for apiVersion and kind properties based on
// x-kubernetes-group-version-kind extension.
func addKCLDefaults(s *spec3.OpenAPI) {
	if s.Components == nil || s.Components.Schemas == nil {
		return
	}

	for _, schema := range s.Components.Schemas {
		processKCLSchemaDefaults(schema)
	}
}

func processKCLSchemaDefaults(schema *spec.Schema) {
	// Look for x-kubernetes-group-version-kind extension
	rawExt, ok := schema.Extensions["x-kubernetes-group-version-kind"]
	if !ok {
		return
	}

	// Convert the extension to a usable format
	gvkList := extractGVKList(rawExt)
	if len(gvkList) == 0 {
		return
	}

	// Extract group, version, and kind from the first GVK
	group, version, kind := extractGVKInfo(gvkList[0])

	// Construct apiVersion
	apiVersion := constructAPIVersion(group, version)

	// Add defaults to properties
	addSchemaPropertyDefaultsKcl(schema, apiVersion, kind)
}

func addSchemaPropertyDefaultsKcl(schema *spec.Schema, apiVersion, kind string) {
	if schema.Properties == nil {
		return
	}

	// Add default to apiVersion property
	if _, ok := schema.Properties["apiVersion"]; ok {
		propSchema := schema.Properties["apiVersion"]
		propSchema.Default = apiVersion
		propSchema.Enum = []interface{}{apiVersion}
		schema.Properties["apiVersion"] = propSchema
	}

	// Add default to kind property
	if _, ok := schema.Properties["kind"]; ok {
		propSchema := schema.Properties["kind"]
		propSchema.Default = kind
		propSchema.Enum = []interface{}{kind}
		schema.Properties["kind"] = propSchema
	}

	// Make apiVersion and kind required
	hasAPIVersion := false
	hasKind := false
	for _, req := range schema.Required {
		if req == "apiVersion" {
			hasAPIVersion = true
		}
		if req == "kind" {
			hasKind = true
		}
	}
	if !hasAPIVersion {
		schema.Required = append(schema.Required, "apiVersion")
	}
	if !hasKind {
		schema.Required = append(schema.Required, "kind")
	}
}
