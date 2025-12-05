// Copyright 2025 Upbound Inc.
// All rights reserved

package example

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	"github.com/spf13/afero"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	v1 "github.com/crossplane/crossplane/v2/apis/apiextensions/v1"
	v2 "github.com/crossplane/crossplane/v2/apis/apiextensions/v2"
	"github.com/crossplane/crossplane/v2/xcrd"

	icrd "github.com/upbound/up/internal/crd"
	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/upterm"
	ixrd "github.com/upbound/up/internal/xrd"
	"github.com/upbound/up/internal/yaml"

	_ "embed"
)

//go:embed help/generate.md
var generateHelp string

func (c *generateCmd) Help() string {
	return generateHelp
}

const (
	outputFile       = "file"
	outputYAML       = "yaml"
	outputJSON       = "json"
	xr               = "Composite Resource (XR)"
	xrString         = "xr"
	xrc              = "Composite Resource Claim (XRC)"
	xrcString        = "xrc"
	claimString      = "claim"
	defaultNamespace = "default"
	scopeCluster     = "cluster"
	scopeNamespace   = "namespace"
)

type resource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              map[string]interface{} `json:"spec"`
}

type generateCmd struct {
	Path   string `help:"Specifies the path to the output file where the  Composite Resource (XR) or Composite Resource Claim (XRC) will be saved." optional:""`
	Output string `default:"file"                                                                                                                   enum:"file,yaml,json" help:"Specifies the output format for the results. Use 'file' to save to a file, 'yaml' to display the  Composite Resource (XR) or Composite Resource Claim (XRC) in YAML format, or 'json' to display in JSON format." short:"o"`

	Type string `default:"" enum:"xr,xrc,claim," help:"Specifies the type of resource to create: 'xrc' for Composite Resource Claim (XRC), 'xr' for Composite Resource (XR)." telemetry:"true"`

	Scope      string `default:""                                         enum:"cluster,namespace," help:"Specifies the XR scope (v2 only)." telemetry:"true"`
	APIGroup   string `help:"Specifies the API group for the resource."`
	APIVersion string `help:"Specifies the API version for the resource."`
	Kind       string `help:"Specifies the Kind of the resource."`
	Name       string `help:"Specifies the Name of the resource."`
	Namespace  string `help:"Specifies the Namespace of the resource."`

	XRDFilePath    string `arg:"" help:"Specifies the path to the Composite Resource Definition (XRD) file used to generate an example resource." optional:""`
	relXrdFilePath string
	ProjectFile    string `default:"upbound.yaml" help:"Path to project definition file." short:"f"`

	projFS    afero.Fs
	exampleFS afero.Fs
	proj      *project.WithVersion
}

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *generateCmd) AfterApply(kongCtx *kong.Context) error {
	ctx := context.Background()

	// Read the project file.
	projFilePath, err := filepath.Abs(c.ProjectFile)
	if err != nil {
		return err
	}
	// The location of the project file defines the root of the project.
	projDirPath := filepath.Dir(projFilePath)
	c.projFS = afero.NewBasePathFs(afero.NewOsFs(), projDirPath)

	// The location of the project file defines the root of the project.
	proj, err := project.ParseWithVersion(c.projFS, filepath.Base(c.ProjectFile))
	if err != nil {
		return err
	}
	proj.Default()

	c.proj = proj

	c.exampleFS = afero.NewBasePathFs(
		c.projFS, proj.Spec.Paths.Examples,
	)

	c.relXrdFilePath = c.XRDFilePath
	if filepath.IsAbs(c.relXrdFilePath) {
		// Convert the absolute path to a relative path within projFS
		projFS, ok := c.projFS.(*afero.BasePathFs)
		if !ok {
			return errors.Errorf("unexpected filesystem type %T for project", projFS)
		}
		relPath, err := filepath.Rel(afero.FullBaseFsPath(projFS, "."), c.relXrdFilePath)
		if err != nil {
			return errors.Wrap(err, "failed to make file path relative to project filesystem")
		}

		// Check if relPath is within c.projFS
		if strings.HasPrefix(relPath, "..") || filepath.IsAbs(relPath) {
			return errors.New("file path is outside the project filesystem")
		}

		c.relXrdFilePath = relPath
	}

	// workaround interfaces not being bindable ref: https://github.com/alecthomas/kong/issues/48
	kongCtx.BindTo(ctx, (*context.Context)(nil))
	return nil
}

func (c *generateCmd) Run() error {
	// For v2 projects, we only have XRs (no XRCs), so set type to xr
	if c.proj != nil && c.proj.IsV2() {
		c.Type = xrString
		c.Scope = c.getInteractiveScope()
	} else if c.Type == "" {
		// For v1 projects, get xr or xrc/claim as input otherwise ask interactive
		c.Type = c.getInteractiveType()
	}
	if len(c.relXrdFilePath) > 0 {
		return c.processXRDFile()
	}
	return c.processInput()
}

// processXRDFile handles the logic when the XRD file path is provided.
func (c *generateCmd) processXRDFile() error {
	xrd, err := c.readXRDFile()
	if err != nil {
		return err
	}

	crd, err := c.createCRDFromXRD(xrd)
	if err != nil {
		return err
	}

	resource, err := c.generateResourceFromCRD(crd)
	if err != nil {
		return err
	}

	return c.outputResource(resource)
}

// readXRDFile reads and unmarshals the XRD file, returning either v1 or v2 XRD.
func (c *generateCmd) readXRDFile() (interface{}, error) {
	xrdRaw, err := afero.ReadFile(c.projFS, c.relXrdFilePath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read file in %s", filesystem.FullPath(c.projFS, c.relXrdFilePath))
	}

	// First, determine the API version
	var typeMeta metav1.TypeMeta
	err = yaml.Unmarshal(xrdRaw, &typeMeta)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal XRD TypeMeta")
	}

	switch typeMeta.APIVersion {
	case v1.CompositeResourceDefinitionGroupVersionKind.GroupVersion().String():
		var xrd v1.CompositeResourceDefinition
		err = yaml.Unmarshal(xrdRaw, &xrd)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal v1 XRD file")
		}
		return xrd, nil
	case v2.CompositeResourceDefinitionGroupVersionKind.GroupVersion().String():
		var xrd v2.CompositeResourceDefinition
		err = yaml.Unmarshal(xrdRaw, &xrd)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal v2 XRD file")
		}
		return xrd, nil
	default:
		return nil, errors.Errorf("unsupported XRD API version: %s", typeMeta.APIVersion)
	}
}

// createCRDFromXRD creates a CRD from the XRD (supports both v1 and v2).
func (c *generateCmd) createCRDFromXRD(xrd interface{}) (*apiextensionsv1.CustomResourceDefinition, error) {
	var crd *apiextensionsv1.CustomResourceDefinition
	var err error
	var xrdName string

	switch x := xrd.(type) {
	case v1.CompositeResourceDefinition:
		xrdName = x.GetName()
		switch c.Type {
		case xrcString, claimString:
			crd, err = xcrd.ForCompositeResourceClaim(&x)
			if err != nil {
				return nil, errors.Wrapf(err, "cannot derive composite CRD from v1 XRD %q for Composite Resource Claim", xrdName)
			}
		case xrString:
			crd, err = xcrd.ForCompositeResource(&x)
			if err != nil {
				return nil, errors.Wrapf(err, "cannot derive composite CRD from v1 XRD %q for Composite Resource", xrdName)
			}
		}
	case v2.CompositeResourceDefinition:
		xrdName = x.GetName()
		// For v2 XRDs, we only support XRs (no XRCs)
		switch c.Type {
		case xrString:
			// Convert v2 XRD to v1 format for xcrd processing
			v1XRD := ixrd.ConvertV2ToV1(&x)
			crd, err = xcrd.ForCompositeResource(v1XRD)
			if err != nil {
				return nil, errors.Wrapf(err, "cannot derive composite CRD from v2 XRD %q for Composite Resource", xrdName)
			}
		default:
			return nil, errors.New("v2 XRDs only support Composite Resources (XRs), not Composite Resource Claims (XRCs)")
		}
	default:
		return nil, errors.New("unsupported XRD type")
	}

	crdGVK := apiextensionsv1.SchemeGroupVersion.WithKind("CustomResourceDefinition")
	crd.SetGroupVersionKind(crdGVK)
	return crd, nil
}

// generateResourceFromCRD generates a resource from a CRD.
func (c *generateCmd) generateResourceFromCRD(crd *apiextensionsv1.CustomResourceDefinition) (resource, error) {
	var res resource

	yamlData, err := icrd.GenerateExample(*crd, true, false)
	if err != nil {
		return res, errors.Wrapf(err, "failed generating example")
	}

	yamlBytes, err := yaml.Marshal(&yamlData)
	if err != nil {
		return res, errors.Wrapf(err, "failed to marshal generated yaml")
	}

	err = yaml.Unmarshal(yamlBytes, &res)
	if err != nil {
		return res, errors.Wrapf(err, "failed to unmarshal generated schema")
	}

	res.ObjectMeta.Name = strings.ToLower(res.Kind)

	// Set namespace based on resource type and CRD scope
	switch c.Type {
	case xrcString, claimString:
		// XRC/Claims are always namespace-scoped
		res.ObjectMeta.Namespace = defaultNamespace
	case xrString:
		// For XRs, check the CRD scope to determine if namespace is needed
		if crd.Spec.Scope == apiextensionsv1.NamespaceScoped {
			res.ObjectMeta.Namespace = defaultNamespace
		}
		// If cluster-scoped, no namespace is set
	}

	return res, nil
}

// processInput handles the logic when the XRD file path is not provided (interactive input).
func (c *generateCmd) processInput() error {
	resourceType, compositeName, apiGroup, apiVersion, name, namespace, err := c.collectInteractiveInput()
	if err != nil {
		return err
	}

	resource, err := c.createResource(resourceType, compositeName, apiGroup, apiVersion, name, namespace)
	if err != nil {
		return errors.Wrap(err, "failed to create xrd")
	}

	return c.outputResource(resource)
}

func (c *generateCmd) collectInteractiveInput() (string, string, string, string, string, string, error) {
	// Collect the resource type, kind, API group, API version, metadata.name and metadata.namespace
	resourceType := c.getInteractiveType()
	return resourceType,
		c.getInteractiveKind(resourceType),
		c.getInteractiveGroup(),
		c.getInteractiveVersion(),
		c.getInteractiveMetadataName(),
		c.getInteractiveMetadataNamespace(resourceType),
		nil
}

// getInteractiveType gets the resource type interactively.
func (c *generateCmd) getInteractiveType() string {
	if c.Type != "" {
		return c.Type
	}

	choice, err := upterm.Selection("What do you want to create?", []string{xrc, xr}, xrc)
	if err != nil {
		pterm.Error.Println("An error occurred while getting choice:", err)
		return ""
	}

	var cType string
	if choice == xrc {
		cType = xrcString
	}

	if choice == xr {
		cType = xrString
	}

	return cType
}

// getInteractiveScope asks whether XR should be cluster scoped or namespace
// scoped.
func (c *generateCmd) getInteractiveScope() string {
	if c.Scope != "" {
		return c.Scope
	}

	wantClusterScoped, err := upterm.Confirm("Should this Composite Resource (XR) be cluster scoped? (default: namespace scoped)", false)
	if err != nil {
		pterm.Error.Println("An error occurred while getting scoping choice:", err)
		return scopeNamespace // Default to namespace scoped.
	}
	if wantClusterScoped {
		return scopeCluster
	}

	return scopeNamespace
}

// getInteractiveKind gets the resource kind interactively.
func (c *generateCmd) getInteractiveKind(resourceType string) string {
	if c.Kind != "" {
		return c.Kind
	}

	prompt := "What is your Composite Resource (XR) kind?"
	def := "Cluster"
	if resourceType == xrcString {
		prompt = "What is your Composite Resource Claim (XRC) kind?"
	} else if c.proj.IsV1() {
		// For V1 projects, use "XCluster" as default for XR.
		def = "XCluster"
	}

	name, err := upterm.Prompt(prompt, def)
	if err != nil {
		pterm.Error.Println("An error occurred while getting Claim or Composite Resource name:", err)
		return ""
	}

	return name
}

// getInteractiveGroup gets the API group interactively.
func (c *generateCmd) getInteractiveGroup() string {
	if c.APIGroup != "" {
		return c.APIGroup
	}

	group, err := upterm.Prompt("What is the API group named?", "customer.upbound.io")
	if err != nil {
		pterm.Error.Println("An error occurred while getting API Group:", err)
		return ""
	}

	return group
}

// getInteractiveVersion gets the API version interactively.
func (c *generateCmd) getInteractiveVersion() string {
	if c.APIVersion != "" {
		return c.APIVersion
	}

	version, err := upterm.Prompt("What is the API Version named?", "v1alpha1")
	if err != nil {
		pterm.Error.Println("An error occurred while getting API Version:", err)
		return ""
	}

	return version
}

// getInteractiveMetadataName gets the metadata.name interactively.
func (c *generateCmd) getInteractiveMetadataName() string {
	if c.Name != "" {
		return c.Name
	}

	name, err := upterm.Prompt("What is the metadata name?", "example")
	if err != nil {
		pterm.Error.Println("An error occurred while getting metadata.name:", err)
		return ""
	}

	return name
}

// getInteractiveMetadataNamespace gets the metadata.namespace interactively.
func (c *generateCmd) getInteractiveMetadataNamespace(resourceType string) string {
	if c.Namespace != "" {
		return c.Namespace
	}

	// For v1 projects: XRC/Claims always ask for namespace, XRs don't have namespace
	// For v2 projects: XRs ask for namespace if they are namespace scoped.
	if resourceType == xrcString || resourceType == claimString || c.Scope == scopeNamespace {
		namespace, err := upterm.Prompt("What is the metadata namespace?", defaultNamespace)
		if err != nil {
			pterm.Error.Println("An error occurred while getting metadata.namespace:", err)
			return ""
		}

		return namespace
	}

	// For XR resources in v1 projects and cluster socped XRs in v2 projects, no
	// namespace.
	return ""
}

// createResource creates a resource based on the collected input.
func (c *generateCmd) createResource(resourceType, compositeName, apiGroup, apiVersion, name, namespace string) (resource, error) {
	var res resource
	// Check if required fields are missing or invalid
	if compositeName == "" {
		return res, errors.New("compositeName is required")
	}
	if apiGroup == "" {
		return res, errors.New("apiGroup is required")
	}
	if resourceType == "" {
		return res, errors.New("resourceType is required")
	}
	if apiVersion == "" || !icrd.IsKnownAPIVersion(apiVersion) {
		return res, fmt.Errorf("apiVersion is required or invalid. Valid versions are: %v", icrd.KnownAPIVersions)
	}
	validatedNamespace, err := validateNameNamespace(name, namespace)
	if err != nil {
		return res, err
	}

	res = resource{
		TypeMeta: metav1.TypeMeta{
			APIVersion: fmt.Sprintf("%s/%s", apiGroup, apiVersion),
			Kind:       compositeName,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: strings.ToLower(name),
		},
		Spec: map[string]interface{}{},
	}

	// Set namespace for XRC/Claims (v1) or for XRs when namespace is provided (v1 and v2)
	if resourceType == xrcString || resourceType == claimString || (resourceType == xrString && namespace != "") {
		res.ObjectMeta.Namespace = strings.ToLower(validatedNamespace)
	}

	return res, nil
}

// outputResource handles the output of the generated resource based on the specified output type.
func (c *generateCmd) outputResource(res resource) error {
	// Convert resource to YAML format
	resourceYAML, err := yaml.Marshal(res)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal resource to YAML")
	}

	switch c.Output {
	case outputFile:
		filePath := c.Path
		if filePath == "" {
			filePath = fmt.Sprintf("%s/%s.yaml", strings.ToLower(res.Kind), strings.ToLower(res.ObjectMeta.Name))
		}

		// Check if the example file already exists
		exists, err := afero.Exists(c.exampleFS, filePath)
		if err != nil {
			return errors.Wrap(err, "failed to check if file exists")
		}

		if exists {
			// Ignore any error, since false will be returned and we'll abort
			// anyway.
			result, _ := upterm.Confirm(
				fmt.Sprintf("The example file '%s' already exists. Do you want to override its contents?", filesystem.FullPath(c.exampleFS, filePath)),
				false,
			)
			if !result {
				return errors.New("operation cancelled by user")
			}
		}

		if err := c.exampleFS.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
			return errors.Wrap(err, "failed to create directories for the specified output path")
		}

		if err := afero.WriteFile(c.exampleFS, filePath, resourceYAML, 0o644); err != nil {
			return errors.Wrap(err, "failed to write example to file")
		}

		pterm.Printfln("Successfully created example and saved to %s", filesystem.FullPath(c.exampleFS, filePath))

	case outputYAML:
		pterm.Println(string(resourceYAML))
	case outputJSON:
		jsonData, err := yaml.YAMLToJSON(resourceYAML)
		if err != nil {
			return errors.Wrapf(err, "failed to convert resource to JSON")
		}
		pterm.Println(string(jsonData))
	default:
		return errors.New("invalid output format specified")
	}

	return nil
}

// validateNameNamespace checks that the name and (if provided) the namespace are valid DNS labels.
func validateNameNamespace(name, namespace string) (string, error) {
	// TODO(adamwg): Replace with validation from k8s validation package.
	dnsLabelRegex := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

	if len(name) > 63 {
		return "", errors.New("metadata.name must be no more than 63 characters")
	}
	if !dnsLabelRegex.MatchString(name) {
		return "", errors.New("metadata.name is invalid: must be a valid DNS label (lowercase alphanumeric, may include hyphens)")
	}

	if namespace == "" {
		namespace = defaultNamespace
	} else {
		if len(namespace) > 63 {
			return "", errors.New("metadata.namespace must be no more than 63 characters")
		}
		if !dnsLabelRegex.MatchString(namespace) {
			return "", errors.New("metadata.namespace is invalid: must be a valid DNS label (lowercase alphanumeric, may include hyphens)")
		}
	}

	return namespace, nil
}
