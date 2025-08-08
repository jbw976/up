// Copyright 2025 Upbound Inc.
// All rights reserved

package xrd

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/gobuffalo/flect"
	"github.com/pterm/pterm"
	"github.com/spf13/afero"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	v2 "github.com/crossplane/crossplane/apis/apiextensions/v2"

	"github.com/upbound/up/internal/crd"
	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/schemas/generator"
	"github.com/upbound/up/internal/schemas/manager"
	"github.com/upbound/up/internal/schemas/runner"
	"github.com/upbound/up/internal/style"
	"github.com/upbound/up/internal/yaml"
)

func (c *generateCmd) Help() string {
	return style.RenderHelp(`
The <generate> command creates a CompositeResourceDefinition (XRD) from a given Composite Resource (XR) and generates associated language models for function usage.

## Usage Examples:

    up xrd generate <examples/cluster/example.yaml>
        Generates a CompositeResourceDefinition (XRD) based on the specified Composite Resource and saves it to the default APIs folder in the project.

    up xrd generate <examples/postgres/example.yaml> --plural <postgreses>
        Generates a CompositeResourceDefinition (XRD) with a specified plural form, useful for cases where automatic pluralization may not be accurate (e.g., "postgres").

    up xrd generate <examples/postgres/example.yaml> --path <database/definition.yaml>
        Generates a CompositeResourceDefinition (XRD) and saves it to a custom path within the project's default APIs folder.
`)
}

const (
	outputFile = "file"
	outputYAML = "yaml"
	outputJSON = "json"
)

type inputYAML struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              map[string]interface{} `json:"spec"`
	Status            map[string]interface{} `json:"status"`
}

// parsedXRD represents the common parsed data from XR YAML.
type parsedXRD struct {
	input        inputYAML
	group        string
	version      string
	kind         string
	plural       string
	description  string
	specProps    map[string]extv1.JSONSchemaProps
	statusProps  map[string]extv1.JSONSchemaProps
	rawSchema    *runtime.RawExtension
	hasNamespace bool
}

type generateCmd struct {
	File     string `arg:""                                                                                      help:"Path to the file containing the Composite Resource (XR) or Composite Resource Claim (XRC)."`
	CacheDir string `default:"~/.up/cache/"                                                                      env:"CACHE_DIR"                                                                                   help:"Directory used for caching dependency images."                                                                                    type:"path"`
	Path     string `help:"Path to the output file where the Composite Resource Definition (XRD) will be saved." optional:""`
	Plural   string `help:"Optional custom plural form for the Composite Resource Definition (XRD)."             optional:""`
	Output   string `default:"file"                                                                              enum:"file,yaml,json"                                                                             help:"Output format for the results: 'file' to save to a file, 'yaml' to print XRD in YAML format, 'json' to print XRD in JSON format." short:"o"`

	ProjectFile string `default:"upbound.yaml" help:"Path to project definition file." short:"f"`

	projFS  afero.Fs
	apisFS  afero.Fs
	proj    *project.WithVersion
	relFile string

	sm *manager.Manager
}

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *generateCmd) AfterApply(kongCtx *kong.Context) error {
	kongCtx.Bind(pterm.DefaultBulletList.WithWriter(kongCtx.Stdout))
	ctx := context.Background()

	// Read the project file.
	projFilePath, err := filepath.Abs(c.ProjectFile)
	if err != nil {
		return err
	}
	// The location of the project file defines the root of the project.
	projDirPath := filepath.Dir(projFilePath)
	c.projFS = afero.NewBasePathFs(afero.NewOsFs(), projDirPath)

	// The location of the co position defines the root of the xrd.
	proj, err := project.ParseWithVersion(c.projFS, filepath.Base(c.ProjectFile))
	if err != nil {
		return err
	}
	proj.Default()

	c.proj = proj

	c.apisFS = afero.NewBasePathFs(
		c.projFS, proj.Spec.Paths.APIs,
	)

	c.sm = manager.New(afero.NewBasePathFs(c.projFS, ".up"), generator.AllLanguages(), runner.NewRealSchemaRunner())

	c.relFile = c.File
	if filepath.IsAbs(c.File) {
		// Convert the absolute path to a relative path within projFS
		relPath, err := filepath.Rel(afero.FullBaseFsPath(c.projFS.(*afero.BasePathFs), "."), c.File) //nolint:forcetypeassert // We know the type of projFS from above.
		if err != nil {
			return errors.Wrap(err, "failed to make file path relative to project filesystem")
		}

		// Check if relPath is within c.projFS
		if strings.HasPrefix(relPath, "..") || filepath.IsAbs(relPath) {
			return errors.New("file path is outside the project filesystem")
		}

		c.relFile = relPath
	}

	// workaround interfaces not being bindable ref: https://github.com/alecthomas/kong/issues/48
	kongCtx.BindTo(ctx, (*context.Context)(nil))
	return nil
}

func (c *generateCmd) Run(ctx context.Context, p pterm.TextPrinter) error {
	yamlData, err := afero.ReadFile(c.projFS, c.relFile)
	if err != nil {
		return errors.Wrapf(err, "failed to read file in %s", filesystem.FullPath(c.projFS, c.relFile))
	}

	var xrd any
	if c.proj.IsV2() {
		xrd, err = newXRDv2(yamlData, c.Plural)
		if err != nil {
			return errors.Wrap(err, "failed to create CompositeResourceDefinition (XRD)")
		}
	} else {
		xrd, err = newXRDv1(yamlData, c.Plural)
		if err != nil {
			return errors.Wrap(err, "failed to create CompositeResourceDefinition (XRD)")
		}
	}

	var pluralName string
	switch x := xrd.(type) {
	case *v1.CompositeResourceDefinition:
		pluralName = x.Spec.Names.Plural
	case *v2.CompositeResourceDefinition:
		pluralName = x.Spec.Names.Plural
	}

	xrdYAML, err := yaml.Marshal(xrd, yaml.RemoveField("status"))
	if err != nil {
		return errors.Wrap(err, "failed to marshal XRD to YAML")
	}

	switch c.Output {
	case outputFile:
		// Determine the file path
		filePath := c.Path
		if filePath == "" {
			filePath = fmt.Sprintf("%s/definition.yaml", pluralName)
		}

		// Check if the composition file already exists
		exists, err := afero.Exists(c.apisFS, filePath)
		if err != nil {
			return errors.Wrap(err, "failed to check if file exists")
		}

		if exists {
			// Prompt the user for confirmation to merge
			pterm.Println() // Blank line for spacing
			confirm := pterm.DefaultInteractiveConfirm
			confirm.DefaultText = fmt.Sprintf("The CompositeResourceDefinition (XRD) file '%s' already exists. Do you want to override its contents?", filesystem.FullPath(c.apisFS, filePath))
			confirm.DefaultValue = false

			result, _ := confirm.Show() // Display confirmation prompt
			pterm.Println()             // Blank line for spacing

			if !result {
				return errors.New("operation cancelled by user")
			}
		}

		if err := c.apisFS.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
			return errors.Wrap(err, "failed to create directories for the specified output path")
		}

		if err := afero.WriteFile(c.apisFS, filePath, xrdYAML, 0o644); err != nil {
			return errors.Wrap(err, "failed to write CompositeResourceDefinition (XRD) to file")
		}

		if err := c.sm.Add(ctx, manager.NewFSSource(c.apisFS)); err != nil {
			return errors.Wrap(err, "failed to generate language schemas")
		}

		p.Printfln("Successfully created CompositeResourceDefinition (XRD) and saved to %s", filesystem.FullPath(c.apisFS, filePath))

	case outputYAML:
		p.Println(string(xrdYAML))

	case outputJSON:
		jsonData, err := yaml.YAMLToJSON(xrdYAML)
		if err != nil {
			return errors.Wrapf(err, "failed to convert XRD to JSON")
		}
		p.Println(string(jsonData))

	default:
		return errors.New("invalid output format specified")
	}

	return nil
}

// parseAndValidateXRD parses and validates the input YAML and returns common XRD data.
func parseAndValidateXRD(yamlData []byte, customPlural string) (*parsedXRD, error) {
	var input inputYAML
	err := yaml.Unmarshal(yamlData, &input)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal YAML")
	}

	// Ensure only allowed top-level keys: apiVersion, kind, metadata, spec, and status
	var topLevelKeys map[string]interface{}
	err = yaml.Unmarshal(yamlData, &topLevelKeys)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal YAML to check top-level keys")
	}
	for key := range topLevelKeys {
		if key != "apiVersion" && key != "kind" && key != "metadata" && key != "spec" && key != "status" {
			return nil, errors.New("invalid manifest: only apiVersion, kind, metadata, spec, and status are allowed as top-level keys")
		}
	}

	if input.APIVersion == "" {
		return nil, errors.New("invalid manifest: apiVersion is required")
	}

	// Check if apiVersion contains exactly one slash (/) to ensure it's in "group/version" format
	if strings.Count(input.APIVersion, "/") != 1 {
		return nil, errors.New("invalid manifest: apiVersion must be in the format group/version")
	}

	if input.Kind == "" {
		return nil, errors.New("invalid manifest: kind is required")
	}
	if input.Name == "" {
		return nil, errors.New("invalid manifest: metadata.name is required")
	}
	if input.Spec == nil {
		return nil, errors.New("invalid manifest: spec is required")
	}

	// List of standard Crossplane fields to remove from the XR/XRC.
	// These fields are automatically populated by Crossplane when the CRD is created in the cluster.
	fieldsToRemove := []string{
		"resourceRefs",
		"writeConnectionSecretToRef",
		"publishConnectionDetailsTo",
		"environmentConfigRefs",
		"compositionUpdatePolicy",
		"compositionRevisionRef",
		"compositionRevisionSelector",
		"compositionRef",
		"compositionSelector",
		"claimRef",
	}

	for _, field := range fieldsToRemove {
		delete(input.Spec, field)
	}

	gv, err := schema.ParseGroupVersion(input.APIVersion)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse API version")
	}

	group := gv.Group
	version := gv.Version
	kind := input.Kind

	// Use custom plural if provided, otherwise generate it
	plural := customPlural
	if plural == "" {
		plural = flect.Pluralize(kind)
	}

	description := fmt.Sprintf("%s is the Schema for the %s API.", kind, kind)

	// Infer properties for spec and status and handle errors
	specProps, err := crd.InferProperties(input.Spec)
	if err != nil {
		return nil, errors.Wrap(err, "failed to infer properties for spec")
	}

	statusProps, err := crd.InferProperties(input.Status)
	if err != nil {
		return nil, errors.Wrap(err, "failed to infer properties for status")
	}

	openAPIV3Schema := &extv1.JSONSchemaProps{
		Description: description,
		Type:        "object",
		Properties: map[string]extv1.JSONSchemaProps{
			"spec": {
				Description: fmt.Sprintf("%sSpec defines the desired state of %s.", kind, kind),
				Type:        "object",
				Properties:  specProps,
			},
			"status": {
				Description: fmt.Sprintf("%sStatus defines the observed state of %s.", kind, kind),
				Type:        "object",
				Properties:  statusProps,
			},
		},
		Required: []string{"spec"},
	}

	// Convert openAPIV3Schema as JSONSchemaProps to a RawExtension
	schemaBytes, err := json.Marshal(openAPIV3Schema)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal OpenAPI v3 schema")
	}

	rawSchema := &runtime.RawExtension{
		Raw: schemaBytes,
	}

	return &parsedXRD{
		input:        input,
		group:        group,
		version:      version,
		kind:         kind,
		plural:       plural,
		description:  description,
		specProps:    specProps,
		statusProps:  statusProps,
		rawSchema:    rawSchema,
		hasNamespace: input.Namespace != "",
	}, nil
}

// newXRDv1 creates a new CompositeResourceDefinition v1.
func newXRDv1(yamlData []byte, customPlural string) (*v1.CompositeResourceDefinition, error) {
	// Parse and validate common XRD data
	parsed, err := parseAndValidateXRD(yamlData, customPlural)
	if err != nil {
		return nil, err
	}

	// For v1: Determine whether to modify based on XRC
	kind := parsed.kind
	plural := parsed.plural
	if parsed.hasNamespace {
		// Ensure plural and kind start with 'x'
		if !strings.HasPrefix(plural, "x") {
			plural = "x" + plural
		}
		if !strings.HasPrefix(kind, "x") {
			kind = "x" + kind
		}
	}

	// Construct the XRD v1
	xrd := &v1.CompositeResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1.CompositeResourceDefinitionGroupVersionKind.GroupVersion().String(),
			Kind:       v1.CompositeResourceDefinitionGroupVersionKind.Kind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: strings.ToLower(fmt.Sprintf("%s.%s", plural, parsed.group)),
		},
		Spec: v1.CompositeResourceDefinitionSpec{
			Group: parsed.group,
			Names: extv1.CustomResourceDefinitionNames{
				Categories: []string{"crossplane"},
				Kind:       flect.Capitalize(kind),
				Plural:     strings.ToLower(plural),
			},
			Versions: []v1.CompositeResourceDefinitionVersion{
				{
					Name:          parsed.version,
					Referenceable: true,
					Served:        true,
					Schema: &v1.CompositeResourceValidation{
						OpenAPIV3Schema: *parsed.rawSchema,
					},
				},
			},
		},
	}

	// For v1: Conditionally add ClaimNames without 'x' prefix if metadata.namespace is present
	if parsed.hasNamespace {
		claimPlural := strings.ToLower(strings.TrimPrefix(plural, "x"))
		claimKind := flect.Capitalize(strings.TrimPrefix(kind, "x"))

		xrd.Spec.ClaimNames = &extv1.CustomResourceDefinitionNames{
			Kind:   claimKind,
			Plural: claimPlural,
		}
	}

	return xrd, nil
}

// newXRDv2 creates a new CompositeResourceDefinition v2.
func newXRDv2(yamlData []byte, customPlural string) (*v2.CompositeResourceDefinition, error) {
	// Parse and validate common XRD data
	parsed, err := parseAndValidateXRD(yamlData, customPlural)
	if err != nil {
		return nil, err
	}

	// For v2: Handle scope based on namespace
	scope := v2.CompositeResourceScopeCluster
	if parsed.hasNamespace {
		scope = v2.CompositeResourceScopeNamespaced
	}

	// Construct the XRD v2
	xrd := &v2.CompositeResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v2.CompositeResourceDefinitionGroupVersionKind.GroupVersion().String(),
			Kind:       v2.CompositeResourceDefinitionGroupVersionKind.Kind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: strings.ToLower(fmt.Sprintf("%s.%s", parsed.plural, parsed.group)),
		},
		Spec: v2.CompositeResourceDefinitionSpec{
			Group: parsed.group,
			Scope: scope,
			Names: extv1.CustomResourceDefinitionNames{
				Categories: []string{"crossplane"},
				Kind:       flect.Capitalize(parsed.kind),
				Plural:     strings.ToLower(parsed.plural),
			},
			Versions: []v2.CompositeResourceDefinitionVersion{
				{
					Name:          parsed.version,
					Referenceable: true,
					Served:        true,
					Schema: &v2.CompositeResourceValidation{
						OpenAPIV3Schema: *parsed.rawSchema,
					},
				},
			},
		},
	}

	return xrd, nil
}
