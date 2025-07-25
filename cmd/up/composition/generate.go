// Copyright 2025 Upbound Inc.
// All rights reserved

package composition

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/gobuffalo/flect"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/pterm/pterm"
	"github.com/spf13/afero"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	apiextv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	pkgv1beta1 "github.com/crossplane/crossplane/apis/pkg/v1beta1"

	xcrd "github.com/upbound/up/internal/crd"
	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/xpkg"
	"github.com/upbound/up/internal/xpkg/dep"
	"github.com/upbound/up/internal/xpkg/dep/cache"
	"github.com/upbound/up/internal/xpkg/dep/manager"
	mxpkg "github.com/upbound/up/internal/xpkg/dep/marshaler/xpkg"
	"github.com/upbound/up/internal/xpkg/dep/resolver/image"
	"github.com/upbound/up/internal/yaml"
	projectv1alpha1 "github.com/upbound/up/pkg/apis/project/v1alpha1"
)

func (c *generateCmd) Help() string {
	return `
The 'generate' command creates a composition and adds the required function packages to the project as dependencies.

Examples:
    composition generate apis/xnetwork/definition.yaml
        Generates a composition from an CompositeResourceDefinition (XRD).
		Saves output to 'apis/xnetworks/composition.yaml'.

    composition generate examples/xnetwork/xnetwork.yaml
        Generates a composition from an Composite Resource (XR).
		Saves output to 'apis/xnetworks/composition.yaml'.

    composition generate examples/network/network-aws.yaml --name aws
        Generates a composition from the Composite Resource Claim (XRC) with labels
		if 'spec.compositionSelector.matchLabels' is set in the XR, using 'aws' as a prefix in 'metadata.name'.
		Saves output to 'apis/xnetworks/composition-aws.yaml'.

    composition generate examples/xnetwork/xnetwork-azure.yaml --name azure
        Generates a composition from the Composite Resource (XR) or Composite Resource Claim (XRC) with labels
		if 'spec.compositionSelector.matchLabels' is set in the XR, using 'azure' as a prefix in 'metadata.name'.
		Saves output to 'apis/xnetworks/composition-azure.yaml'.

    composition generate examples/xdatabase/database.yaml --plural postgreses
        Generates a composition from the Composite Resource (XR) with a custom plural form,
		Saves output to 'apis/xdatabases/composition.yaml'.
`
}

const (
	outputFile            = "file"
	outputYAML            = "yaml"
	outputJSON            = "json"
	errInvalidPkgName     = "invalid package dependency supplied"
	functionAutoReadyXpkg = "xpkg.upbound.io/crossplane-contrib/function-auto-ready"
	kclTemplate           = `{
		"apiVersion": "template.fn.crossplane.io/v1beta1",
		"kind": "KCLInput",
		"spec": {
			"source": ""
		}
	}`

	goTemplate = `{
		"apiVersion": "gotemplating.fn.crossplane.io/v1beta1",
		"kind": "GoTemplate",
		"source": "Inline",
		"inline": {
			"template": ""
		}
	}`

	patTemplate = `{
		"apiVersion": "pt.fn.crossplane.io/v1beta1",
		"kind": "Resources",
		"resources": []
	}`
)

type generateCmd struct {
	Resource string `arg:""                                                      help:"File path to Composite Resource Claim (XRC) or Composite Resource (XR) or CompositeResourceDefinition (XRD)." required:""`
	Name     string `help:"Name for the new composition."                        optional:""`
	Plural   string `help:"Optional custom plural for the CompositeTypeRef.Kind" optional:""`

	Path        string `help:"Optional path to the output file where the generated Composition will be saved." optional:""`
	ProjectFile string `default:"upbound.yaml"                                                                 help:"Path to project definition file." short:"f"`
	Output      string `default:"file"                                                                         enum:"file,yaml,json"                   help:"Output format for the results: 'file' to save to a file, 'yaml' to print XRD in YAML format, 'json' to print XRD in JSON format." short:"o"`
	CacheDir    string `default:"~/.up/cache/"                                                                 env:"CACHE_DIR"                         help:"Directory used for caching dependency images."                                                                                    type:"path"`

	Flags upbound.Flags `embed:""`

	projFS afero.Fs
	apisFS afero.Fs
	proj   *projectv1alpha1.Project

	m *manager.Manager
}

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *generateCmd) AfterApply(kongCtx *kong.Context) error {
	kongCtx.Bind(pterm.DefaultBulletList.WithWriter(kongCtx.Stdout))
	ctx := context.Background()

	upCtx, err := upbound.NewFromFlags(c.Flags)
	if err != nil {
		return err
	}

	// Read the project file.
	projFilePath, err := filepath.Abs(c.ProjectFile)
	if err != nil {
		return err
	}
	// The location of the project file defines the root of the project.
	projDirPath := filepath.Dir(projFilePath)
	c.projFS = afero.NewBasePathFs(afero.NewOsFs(), projDirPath)

	// parse the project and apply defaults.
	proj, err := project.Parse(c.projFS, c.ProjectFile)
	if err != nil {
		return err
	}
	proj.Default()

	c.proj = proj

	c.apisFS = afero.NewBasePathFs(
		c.projFS, proj.Spec.Paths.APIs,
	)

	fs := afero.NewOsFs()

	cache, err := cache.NewLocal(c.CacheDir, cache.WithFS(fs))
	if err != nil {
		return err
	}

	r := image.NewResolver(
		image.WithImageConfig(proj.Spec.ImageConfig),
		image.WithFetcher(
			image.NewLocalFetcher(
				image.WithKeychain(upCtx.RegistryKeychain()),
			),
		),
	)

	m, err := manager.New(
		manager.WithCache(cache),
		manager.WithResolver(r),
		manager.WithSkipCacheUpdateIfExists(true),
	)
	if err != nil {
		return err
	}

	c.m = m

	// workaround interfaces not being bindable ref: https://github.com/alecthomas/kong/issues/48
	kongCtx.BindTo(ctx, (*context.Context)(nil))
	return nil
}

func (c *generateCmd) Run(ctx context.Context, p pterm.TextPrinter) error { //nolint:gocyclo // multiple output options
	composition, plural, err := c.newComposition(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create composition")
	}

	// Convert Composition to YAML format
	compositionYAML, err := yaml.Marshal(composition)
	if err != nil {
		return errors.Wrap(err, "failed to marshal composition to yaml")
	}

	switch c.Output {
	case outputFile:
		// Determine the file path
		filePath := c.Path
		if filePath == "" {
			if c.Name != "" {
				filePath = fmt.Sprintf("%s/composition-%s.yaml", strings.ToLower(plural), c.Name)
			} else {
				filePath = fmt.Sprintf("%s/composition.yaml", strings.ToLower(plural))
			}
		}

		// Check if the composition already exists
		exists, err := afero.Exists(c.apisFS, filePath)
		if err != nil {
			return errors.Wrap(err, "failed to check if file exists")
		}

		// If the file exists, prompt the user for confirmation to overwrite
		if exists {
			pterm.Println() // Blank line for spacing
			confirm := pterm.DefaultInteractiveConfirm

			baseApisFS, ok := c.apisFS.(*afero.BasePathFs)
			if !ok {
				return errors.New("failed to construct the path")
			}
			confirm.DefaultText = fmt.Sprintf("The Composition file '%s' already exists. Do you want to overwrite its contents?", afero.FullBaseFsPath(baseApisFS, filePath))
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

		// Write the YAML to the specified output file
		if err := afero.WriteFile(c.apisFS, filePath, compositionYAML, 0o644); err != nil {
			return errors.Wrap(err, "failed to write composition to file")
		}

		baseApisFS, ok := c.apisFS.(*afero.BasePathFs)
		if !ok {
			return fmt.Errorf("failed to assert c.apisFS to *afero.BasePathFs")
		}
		p.Printfln("successfully created Composition and saved to %s", afero.FullBaseFsPath(baseApisFS, filePath))

	case outputYAML:
		p.Println(string(compositionYAML))

	case outputJSON:
		jsonData, err := yaml.YAMLToJSON(compositionYAML)
		if err != nil {
			return errors.Wrap(err, "failed to convert composition to JSON")
		}
		p.Println(string(jsonData))

	default:
		return errors.New("invalid output format specified")
	}

	return nil
}

// newComposition to create a new Composition.
func (c *generateCmd) newComposition(ctx context.Context) (*apiextv1.Composition, string, error) { //nolint:gocyclo // construct the composition
	group, version, kind, plural, matchLabels, err := c.processResource()
	if err != nil {
		return nil, "", errors.Wrap(err, "failed to load resource")
	}

	// Use custom name if provided, otherwise generate it
	name := c.Name
	if name == "" {
		name = strings.ToLower(fmt.Sprintf("%s.%s", plural, group))
	} else {
		name = strings.ToLower(fmt.Sprintf("%s.%s.%s", c.Name, plural, group))
	}

	pipelineSteps, err := c.createPipelineFromProject(ctx)
	if err != nil {
		return nil, "", errors.Wrap(err, "failed create pipelines from project")
	}

	composition := &apiextv1.Composition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiextv1.CompositionGroupVersionKind.GroupVersion().String(),
			Kind:       apiextv1.CompositionGroupVersionKind.Kind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: apiextv1.CompositionSpec{
			CompositeTypeRef: apiextv1.TypeReference{
				APIVersion: fmt.Sprintf("%s/%s", group, version),
				Kind:       kind,
			},
			Mode:     ptr.To(apiextv1.CompositionModePipeline),
			Pipeline: pipelineSteps,
		},
	}

	if len(matchLabels) > 0 {
		composition.Labels = matchLabels
	}

	return composition, plural, nil
}

func (c *generateCmd) createPipelineFromProject(ctx context.Context) ([]apiextv1.PipelineStep, error) { //nolint:gocognit // ToDo(haarchri): refactor this
	maxSteps := len(c.proj.Spec.DependsOn)
	pipelineSteps := make([]apiextv1.PipelineStep, 0, maxSteps)
	foundAutoReadyFunction := false

	var deps []*mxpkg.ParsedPackage
	var err error

	for _, dep := range c.proj.Spec.DependsOn {
		ref, ok := functionFromDep(dep)
		if !ok {
			continue
		}

		functionName, err := name.NewRepository(ref, name.StrictValidation)
		if err != nil {
			return nil, errors.Wrap(err, errInvalidPkgName)
		}

		// Check if auto-ready-function is available in deps
		if functionName.String() == functionAutoReadyXpkg {
			foundAutoReadyFunction = true
		}

		// Convert the dependency to v1beta1.Dependency
		convertedDep, ok := manager.ConvertToV1beta1(dep)
		if !ok {
			return nil, errors.Errorf("failed to convert dependency in %s", functionName)
		}

		// Try to resolve the function
		_, deps, err = c.m.Resolve(ctx, convertedDep)
		if err != nil {
			// If resolving fails, try to add function
			_, deps, err = c.m.AddAll(ctx, convertedDep)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to add dependencies in %s", functionName)
			}
		}
	}

	if !foundAutoReadyFunction {
		d := dep.New(functionAutoReadyXpkg)

		var ud pkgv1beta1.Dependency
		ud, deps, err = c.m.AddAll(ctx, d)
		if err != nil {
			return nil, errors.Wrap(err, "failed to add auto-ready dependency")
		}

		metaDep := dep.ToMetaDependency(ud)
		if err := project.UpsertDependency(c.proj, metaDep); err != nil {
			return nil, errors.Wrap(err, "failed to add function-auto-ready dependency")
		}
		if err := project.Update(c.projFS, c.ProjectFile, func(p *projectv1alpha1.Project) {
			p.Spec.DependsOn = c.proj.Spec.DependsOn
		}); err != nil {
			return nil, errors.Wrap(err, "failed to update project dependencies")
		}
	}

	for _, dep := range deps {
		var rawExtension *runtime.RawExtension
		if len(dep.Objs) > 0 {
			kind := dep.Objs[0].GetObjectKind().GroupVersionKind()
			if kind.Kind == "CustomResourceDefinition" && kind.GroupVersion().String() == "apiextensions.k8s.io/v1" {
				if crd, ok := dep.Objs[0].(*apiextensionsv1.CustomResourceDefinition); ok {
					rawExtension, err = c.setRawExtension(*crd)
					if err != nil {
						return nil, errors.Wrap(err, "failed to generate rawExtension for input")
					}
				} else {
					return nil, errors.New("failed to use to CustomResourceDefinition")
				}
			}
		}

		functionName, err := name.NewRepository(dep.DepName, name.StrictValidation)
		if err != nil {
			return nil, errors.Wrap(err, errInvalidPkgName)
		}

		// create a PipelineStep for each function
		step := apiextv1.PipelineStep{
			Step: xpkg.ToDNSLabel(functionName.RepositoryStr()),
			FunctionRef: apiextv1.FunctionReference{
				Name: xpkg.ToDNSLabel(functionName.RepositoryStr()),
			},
		}
		if rawExtension != nil {
			step.Input = rawExtension
		}

		pipelineSteps = append(pipelineSteps, step)
	}

	if len(pipelineSteps) == 0 {
		return nil, errors.New("no function found")
	}

	return reorderPipelineSteps(pipelineSteps), nil
}

func functionFromDep(dep pkgmetav1.Dependency) (string, bool) {
	if dep.Function != nil {
		return *dep.Function, true
	}

	if ptr.Deref(dep.APIVersion, "") == pkgv1.FunctionGroupVersionKind.GroupVersion().String() && ptr.Deref(dep.Kind, "") == pkgv1.FunctionKind {
		return ptr.Deref(dep.Package, ""), true
	}

	return "", false
}

// reorderPipelineSteps ensures the step with functionref.name == "crossplane-contrib-function-auto-ready" is the last one.
func reorderPipelineSteps(pipelineSteps []apiextv1.PipelineStep) []apiextv1.PipelineStep {
	var reorderedSteps []apiextv1.PipelineStep
	var autoReadyStep *apiextv1.PipelineStep

	// Iterate through the steps and separate the "crossplane-contrib-function-auto-ready" step
	for _, step := range pipelineSteps {
		// Create a copy of step to avoid memory aliasing issues
		currentStep := step
		if step.FunctionRef.Name == "crossplane-contrib-function-auto-ready" {
			autoReadyStep = &currentStep
		} else {
			reorderedSteps = append(reorderedSteps, currentStep)
		}
	}

	// If we found the auto-ready step, append it at the end
	if autoReadyStep != nil {
		reorderedSteps = append(reorderedSteps, *autoReadyStep)
	}

	return reorderedSteps
}

func (c *generateCmd) setRawExtension(crd apiextensionsv1.CustomResourceDefinition) (*runtime.RawExtension, error) { //nolint:gocyclo // find the input for functions
	var rawExtensionContent string

	// Get the version using the modified getVersion function
	version, err := xcrd.GetCRDVersion(crd)
	if err != nil {
		return nil, err
	}

	gvk := fmt.Sprintf("%s/%s/%s", crd.Spec.Group, version, crd.Spec.Names.Kind)

	// match GVK and inputType to create the appropriate raw extension content
	switch gvk {
	case "template.fn.crossplane.io/v1beta1/KCLInput":
		rawExtensionContent = kclTemplate

	case "gotemplating.fn.crossplane.io/v1beta1/GoTemplate":
		rawExtensionContent = goTemplate

	case "pt.fn.crossplane.io/v1beta1/Resources":
		rawExtensionContent = patTemplate
	default:
		// nothing matches so we generate the default required fields
		// only required fields from function crd
		yamlData, err := xcrd.GenerateExample(crd, true, false)
		if err != nil {
			return nil, errors.Wrap(err, "failed generating schema")
		}
		jsonData, err := json.Marshal(yamlData)
		if err != nil {
			return nil, errors.Wrap(err, "failed marshaling to JSON")
		}
		rawExtensionContent = string(jsonData)
	}

	raw := &runtime.RawExtension{
		Raw: []byte(rawExtensionContent),
	}

	return raw, nil
}

func (c *generateCmd) processResource() (string, string, string, string, map[string]string, error) {
	resourceRaw, err := afero.ReadFile(c.projFS, c.Resource)
	if err != nil {
		return "", "", "", "", nil, errors.Wrap(err, "failed to read resource file")
	}

	// Create an unstructured object
	var unstructuredObj unstructured.Unstructured
	if err := yaml.Unmarshal(resourceRaw, &unstructuredObj.Object); err != nil {
		return "", "", "", "", nil, errors.Wrap(err, "failed to unmarshal resource into unstructured object")
	}

	// Check if obj is a CompositeResourceDefinition by examining its "kind"
	if unstructuredObj.GetKind() == "CompositeResourceDefinition" {
		// Convert unstructured to CompositeResourceDefinition
		var xrd apiextv1.CompositeResourceDefinition
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, &xrd); err != nil {
			return "", "", "", "", nil, errors.Wrap(err, "failed to convert unstructured object to CompositeResourceDefinition")
		}

		// Successfully identified as CompositeResourceDefinition, extract fields
		group := xrd.Spec.Group
		version, err := xcrd.GetXRDVersion(xrd)
		if err != nil {
			return "", "", "", "", nil, errors.Wrap(err, "failed to retrieve XRD version")
		}
		kind := xrd.Spec.Names.Kind
		plural := xrd.Spec.Names.Plural

		return group, version, kind, plural, nil, nil
	}

	// Fallback: If not a CompositeResourceDefinition, process generically
	gvk := unstructuredObj.GroupVersionKind()
	plural := c.Plural
	if plural == "" {
		plural = flect.Pluralize(gvk.Kind)
	}

	// Attempt to extract matchLabels from spec.compositionSelector.matchLabels
	matchLabels := map[string]string{}
	labels, found, err := unstructured.NestedStringMap(unstructuredObj.Object, "spec", "compositionSelector", "matchLabels")
	if err != nil {
		return "", "", "", "", nil, errors.Wrap(err, "failed to extract matchLabels from resource spec")
	}
	if found {
		matchLabels = labels
	}

	// Return the gathered information along with any matchLabels
	return gvk.Group, gvk.Version, gvk.Kind, plural, matchLabels, nil
}
