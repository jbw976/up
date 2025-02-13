// Copyright 2025 Upbound Inc.
// All rights reserved

package schemagenerator

import (
	"context"
	"encoding/json"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/codegen"
	"github.com/spf13/afero"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kube-openapi/pkg/spec3"
	"k8s.io/kube-openapi/pkg/validation/spec"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	xpv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"

	"github.com/upbound/up/internal/crd"
	"github.com/upbound/up/internal/xpkg/schemarunner"
)

// goModContents is the contents of the go.mod we write for our generated models
// module. All generated models share the same module so that we can generate a
// single dependency from embedded Go functions. We always resolve this
// dependency via a replace statement, so `dev.upbound.io/models` is never
// actually used as a URL, just an identifier.
const goModContents = `module dev.upbound.io/models

go 1.23
`

// GenerateSchemaGo generates Go schemas for the CRDs in the given filesystem.
func GenerateSchemaGo(_ context.Context, fromFS afero.Fs, exclude []string, _ schemarunner.SchemaRunner) (afero.Fs, error) {
	openAPIs, err := goCollectOpenAPIs(fromFS, exclude)
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
	modf, err := schemaFS.Create("models/go.mod")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create go.mod")
	}
	if _, err := modf.WriteString(goModContents); err != nil {
		return nil, errors.Wrap(err, "failed to write go.mod")
	}

	// Extract shared k8s schemas and generate a single set of models for
	// them. We have to do this before generating the other models below because
	// the code below replaces the k8s models with references to these shared
	// ones in-place in the spec.
	k8sSpec := &spec3.OpenAPI{
		Version: "3.0.0",
		Components: &spec3.Components{
			Schemas: map[string]*spec.Schema{},
		},
	}
	for _, oapi := range openAPIs {
		k8sSchemas := goExtractK8sSchemas(oapi.spec)
		for name, schema := range k8sSchemas {
			k8sSpec.Components.Schemas[name] = schema
		}
	}
	code, err := generateGo(k8sSpec, "v1",
		goRenameTypes,
		goReplaceNumberWithInt,
		goRemoveRequired,
	)
	if err != nil {
		return nil, err
	}
	if err := writeGoCode(schemaFS, "meta.k8s.io", "meta", "v1", code); err != nil {
		return nil, err
	}

	// Generate models for the non-k8s schemas.
	for _, oapi := range openAPIs {
		code, err := generateGo(oapi.spec, oapi.version,
			goRenameTypes,
			goReplaceNumberWithInt,
			goRemoveRequired,
			goReferenceK8sTypes,
			goRemoveK8s,
			goKeepOnlyComponents,
		)
		if err != nil {
			return nil, err
		}

		if err := writeGoCode(schemaFS, oapi.crd.Spec.Group, oapi.crd.Spec.Names.Kind, oapi.version, code); err != nil {
			return nil, err
		}
	}

	return schemaFS, nil
}

type goOpenAPI struct {
	crd     *extv1.CustomResourceDefinition
	version string
	spec    *spec3.OpenAPI
}

func goCollectOpenAPIs(fromFS afero.Fs, exclude []string) ([]goOpenAPI, error) { //nolint:gocognit // Hard to split this up, and it's not too long to read.
	crdFS := afero.NewMemMapFs()
	baseFolder := "workdir"

	if err := crdFS.MkdirAll(baseFolder, 0o755); err != nil {
		return nil, err
	}

	var openAPIs []goOpenAPI
	return openAPIs, afero.Walk(fromFS, "/", func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip excluded files
		if !info.IsDir() {
			for _, excl := range exclude {
				if info.Name() == excl {
					return nil // Skip this file
				}
			}
		}

		// Skip excluded paths
		for _, excl := range exclude {
			if strings.HasPrefix(path, excl) {
				return filepath.SkipDir
			}
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
			xrPath, claimPath, err := crd.ProcessXRD(crdFS, bs, path, baseFolder)
			if err != nil {
				return err
			}

			if xrPath != "" {
				bs, err := afero.ReadFile(crdFS, xrPath)
				if err != nil {
					return errors.Wrapf(err, "failed to read file %q", path)
				}

				var c extv1.CustomResourceDefinition
				if err := yaml.Unmarshal(bs, &c); err != nil {
					return errors.Wrapf(err, "failed to unmarshal CRD file %q", path)
				}

				oapis, err := crd.ToOpenAPI(&c)
				if err != nil {
					return err
				}
				for version, oapi := range oapis {
					openAPIs = append(openAPIs, goOpenAPI{spec: oapi, version: version, crd: &c})
				}
			}
			if claimPath != "" {
				bs, err := afero.ReadFile(crdFS, claimPath)
				if err != nil {
					return errors.Wrapf(err, "failed to read file %q", path)
				}

				var c extv1.CustomResourceDefinition
				if err := yaml.Unmarshal(bs, &c); err != nil {
					return errors.Wrapf(err, "failed to unmarshal CRD file %q", path)
				}

				oapis, err := crd.ToOpenAPI(&c)
				if err != nil {
					return err
				}
				for version, oapi := range oapis {
					openAPIs = append(openAPIs, goOpenAPI{spec: oapi, version: version, crd: &c})
				}
			}

		case "CustomResourceDefinition":
			var c extv1.CustomResourceDefinition
			if err := yaml.Unmarshal(bs, &c); err != nil {
				return errors.Wrapf(err, "failed to unmarshal CRD file %q", path)
			}

			oapis, err := crd.ToOpenAPI(&c)
			if err != nil {
				return err
			}
			for version, oapi := range oapis {
				openAPIs = append(openAPIs, goOpenAPI{spec: oapi, version: version, crd: &c})
			}
		}
		return nil
	})
}

func generateGo(s *spec3.OpenAPI, version string, mutators ...func(*spec3.OpenAPI)) (string, error) {
	for _, mut := range mutators {
		mut(s)
	}

	// Round-trip through JSON to convert the spec to the kin library used by
	// oapi-codegen.
	bs, err := json.Marshal(s)
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal OpenAPI spec")
	}
	ld := openapi3.NewLoader()
	oapiInput, err := ld.LoadFromData(bs)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse OpenAPI spec")
	}

	// Generate code!
	goCode, err := codegen.Generate(oapiInput, codegen.Configuration{
		PackageName: version,
		Generate: codegen.GenerateOptions{
			Models: true,
		},
		OutputOptions: codegen.OutputOptions{
			SkipPrune: true,
		},
	})
	if err != nil {
		return "", errors.Wrap(err, "failed to generate go code from OpenAPI schema")
	}

	return goCode, nil
}

// goExtractK8sSchemas returns all k8s meta/v1 schemas from the given OpenAPI
// spec.
func goExtractK8sSchemas(s *spec3.OpenAPI) map[string]*spec.Schema {
	ret := make(map[string]*spec.Schema)
	for name, schema := range s.Components.Schemas {
		if strings.Contains(name, "io.k8s.apimachinery.pkg.apis.meta.v1") {
			ret[name] = schema
		}
	}

	return ret
}

func writeGoCode(schemaFS afero.Fs, group, kind, version, code string) error {
	goPath := filepath.Join("models", goSchemaPath(group, kind, version))
	dir := filepath.Dir(goPath)
	if err := schemaFS.MkdirAll(dir, 0o755); err != nil {
		return errors.Wrap(err, "failed to create directory for schemas")
	}

	f, err := schemaFS.Create(goPath)
	if err != nil {
		return errors.Wrap(err, "failed to create go schema file")
	}
	if _, err := f.WriteString(code); err != nil {
		return errors.Wrap(err, "failed to write go code to file")
	}
	_ = f.Close()

	return nil
}

func goSchemaPath(group, kind, version string) string {
	// Our Go files will live in directories based on the CRD group and
	// version. The filename is the singular kind of the CRD.
	//
	// Example: Kind "Bucket" in group "platform.example.com/v1alpha1" becomes
	// com/example/platform/v1alpha1/bucket.go.
	path := strings.Split(group, ".")
	slices.Reverse(path)
	path = append(path, version, strings.ToLower(kind)+".go")

	return filepath.Join(path...)
}

// goRenameTypes adds annotations to schemas to cause oapi-codegen to generate
// nice type names.
func goRenameTypes(s *spec3.OpenAPI) {
	for name, schema := range s.Components.Schemas {
		goName := goFixName(name)
		if goName == "" {
			delete(s.Components.Schemas, name)
		}
		goRenameSchemaType(goName, schema)
		goRenamePropertyTypes(goName, schema.Properties)
	}
}

func goRenamePropertyTypes(baseName string, props map[string]spec.Schema) {
	for name, prop := range props {
		goName := goFixName(baseName + strings.ToUpper(string(name[0])) + name[1:])

		goRenameSchemaType(goName, &prop)
		goRenamePropertyTypes(goName, prop.Properties)

		if prop.Items != nil {
			goRenameSchemaType(goName, prop.Items.Schema)
			goRenamePropertyTypes(goName, prop.Items.Schema.Properties)
		}

		props[name] = prop
	}
}

func goFixName(name string) string {
	lastDot := strings.LastIndex(name, ".")
	if lastDot == -1 {
		return name
	}
	genName := codegen.SchemaNameToTypeName(name)
	prefix := codegen.SchemaNameToTypeName(name[:lastDot])
	return strings.TrimPrefix(genName, prefix)
}

func goRenameSchemaType(name string, schema *spec.Schema) {
	schema.AddExtension("x-go-type-name", name)
	schema.AddExtension("x-oapi-codegen-only-honour-go-name", true)
}

// goReplaceNumberWithInt adds annotations to schemas to cause oapi-codegen to
// generate int type fields instead of floats for numbers.
func goReplaceNumberWithInt(s *spec3.OpenAPI) {
	for _, schema := range s.Components.Schemas {
		goRetypeSchema(schema, "number", "int")
		goRetypeProperties(schema.Properties, "number", "int")
	}
}

func goRetypeProperties(props map[string]spec.Schema, oldType, newType string) {
	for name, prop := range props {
		goRetypeSchema(&prop, oldType, newType)
		if prop.Items != nil {
			goRetypeSchema(prop.Items.Schema, oldType, newType)
			goRetypeProperties(prop.Items.Schema.Properties, oldType, newType)
		}
		props[name] = prop
	}
}

func goRetypeSchema(schema *spec.Schema, oldType, newType string) {
	if schema.Type.Contains(oldType) {
		schema.AddExtension("x-go-type", newType)
	}
}

// goRemoveRequired removes the required fields from schemas. We want all fields
// in our generated models to be optional (so functions can set only the fields
// they wish to own).
func goRemoveRequired(s *spec3.OpenAPI) {
	for _, schema := range s.Components.Schemas {
		schema.Required = nil
		goRemovePropertiesRequired(schema.Properties)
		if schema.Items != nil {
			goRemovePropertiesRequired(schema.Items.Schema.Properties)
		}
	}
}

func goRemovePropertiesRequired(props map[string]spec.Schema) {
	for name, prop := range props {
		prop.Required = nil
		goRemovePropertiesRequired(prop.Properties)
		if prop.Items != nil {
			prop.Items.Schema.Required = nil
			goRemovePropertiesRequired(prop.Items.Schema.Properties)
		}

		props[name] = prop
	}
}

// goReferenceK8sTypes converts all references to k8s meta/v1 schemas in the
// given spec to references to the shared Go models we generate for the k8s
// schemas.
func goReferenceK8sTypes(s *spec3.OpenAPI) {
	for _, schema := range s.Components.Schemas {
		goReferenceK8sType(schema)
		goReferenceK8sTypesProperties(schema.Properties)
	}
}

func goReferenceK8sType(schema *spec.Schema) {
	ref := schema.Ref.String()
	if strings.Contains(ref, "io.k8s.apimachinery.pkg.apis.meta.v1") {
		tryReplaceK8sType(schema, ref)
	}
	for _, one := range schema.AllOf {
		ref := one.Ref.String()
		if strings.Contains(ref, "io.k8s.apimachinery.pkg.apis.meta.v1") {
			tryReplaceK8sType(schema, ref)
			schema.AllOf = nil
		}
	}
}

func goReferenceK8sTypesProperties(props map[string]spec.Schema) {
	for name, prop := range props {
		goReferenceK8sType(&prop)
		goReferenceK8sTypesProperties(prop.Properties)
		if prop.Items != nil {
			goReferenceK8sTypesProperties(prop.Items.Schema.Properties)
		}

		props[name] = prop
	}
}

func tryReplaceK8sType(schema *spec.Schema, ref string) {
	lastDot := strings.LastIndex(ref, ".")
	if lastDot == -1 {
		return
	}
	t := ref[lastDot+1:]
	schema.AddExtension("x-go-type", "metav1."+t)
	schema.AddExtension("x-go-type-import", map[string]string{
		"path": "dev.upbound.io/models/io/k8s/meta/v1",
		"name": "metav1",
	})
}

// goRemoveK8s removes all k8s meta/v1 schemas from the given OpenAPI spec, so
// that we can generate models for them separately and share them across all our
// other generated models.
func goRemoveK8s(s *spec3.OpenAPI) {
	for name := range s.Components.Schemas {
		if strings.HasPrefix(name, "io.k8s.apimachinery.pkg.apis.meta.v1") {
			delete(s.Components.Schemas, name)
		}
	}
}

// goKeepOnlyComponents leaves only the "components" portion of the OpenAPI spec
// in place. This lets us make oapi-codegen generate code only for schemas and
// not a full REST client.
func goKeepOnlyComponents(s *spec3.OpenAPI) {
	*s = spec3.OpenAPI{
		Version:    s.Version,
		Info:       s.Info,
		Components: s.Components,
	}
}
