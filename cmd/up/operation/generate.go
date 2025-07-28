// Copyright 2025 Upbound Inc.
// All rights reserved

package operation

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/pterm/pterm"
	"github.com/spf13/afero"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	opsv1alpha1 "github.com/crossplane/crossplane/apis/ops/v1alpha1"
	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"

	icrd "github.com/upbound/up/internal/crd"
	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/xpkg"
	"github.com/upbound/up/internal/yaml"
	projectapis "github.com/upbound/up/pkg/apis/project"
	projectv2alpha1 "github.com/upbound/up/pkg/apis/project/v2alpha1"
)

func (c *generateCmd) Help() string {
	return `
The 'generate' command creates a new, empty operation.


Examples:
    up operation generate my-operation
        Generates a new, no-op operation named 'my-operation'.

    up operation generate my-operation --cron "0 0 * * *"
        Generates a new, no-op cron operation named 'my-operation' triggered by a cron schedule.

    up operation generate my-operation --watch-group-version-kind "apps/v1/Deployment" --watch-namespace "my-namespace"
        Generates a new, no-op watch operation named 'my-operation' triggered by a watch on a deployment in namespace 'my-namespace'.

    up operation generate claude-pod-watcher --watch-group-version-kind "apps/v1/Pod" --watch-namespace "default" --functions xpkg.upbound.io/upbound/function-claude
        Generates a new operation named 'claude-pod-watcher' that invokes a Claude prompt when pods change.
`
}

const (
	outputFile = "file"
	outputYAML = "yaml"
	outputJSON = "json"
)

type watchSpec struct {
	MatchLabels      map[string]string `help:"Labels to match on the resource."           name:"labels"                                                            optional:""`
	GroupVersionKind string            `aliases:"gvk"                                     help:"The GVK of resources to watch. For example, 'apps/v1/Deployment'." name:"group-version-kind"`
	Namespace        string            `help:"The namespace in which to watch resources." optional:""`

	gvk schema.GroupVersionKind
}

type generateCmd struct {
	Name string `arg:"" help:"Name for the new operation."`

	Path        string `help:"Optional path to the output file where the generated Operation will be saved." optional:""`
	ProjectFile string `default:"upbound.yaml"                                                               help:"Path to project definition file." short:"f"`
	Output      string `default:"file"                                                                       enum:"file,yaml,json"                   help:"Output format for the results: 'file' to save to a file, 'yaml' to print the Operation in YAML format, 'json' to print the operation in JSON format." short:"o"`
	CacheDir    string `default:"~/.up/cache/"                                                               env:"CACHE_DIR"                         help:"Directory used for caching dependency images."                                                                                                        type:"path"`

	Cron      string    `help:"Cron schedule for the operation."                     optional:""`
	Watch     watchSpec `embed:""                                                    help:"Watch for resources and trigger the operation when they change."                  optional:"" prefix:"watch-"`
	Functions []string  `default:"xpkg.upbound.io/crossplane-contrib/function-dummy" help:"Comma-separated list of functions to call in the generated operation's pipeline."`

	Flags upbound.Flags `embed:""`

	projFS afero.Fs
	opsFS  afero.Fs
	proj   *projectv2alpha1.Project

	depManager *project.DependencyManager
}

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *generateCmd) AfterApply(kongCtx *kong.Context) error {
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
	vproj, err := projectapis.ParseVersioned(c.projFS, c.ProjectFile)
	if err != nil {
		return err
	}
	if vproj.IsV1() {
		return errors.New("operations are supported only in v2alpha1 projects; use `up project upgrade` to update your project")
	}
	c.proj = vproj.V2
	c.proj.Default()

	c.opsFS = afero.NewBasePathFs(
		c.projFS, c.proj.Spec.Paths.Operations,
	)

	dm, err := project.NewDependencyManager(upCtx, c.proj, c.projFS,
		project.WithCacheFS(afero.NewBasePathFs(afero.NewOsFs(), c.CacheDir)),
	)
	if err != nil {
		return err
	}
	c.depManager = dm

	if c.Watch.GroupVersionKind != "" {
		idx := strings.LastIndex(c.Watch.GroupVersionKind, "/")
		if idx < 0 {
			return errors.New("invalid GroupVersionKind; must be of the format: group/version/kind")
		}
		gvStr := c.Watch.GroupVersionKind[:idx]
		kind := c.Watch.GroupVersionKind[idx+1:]

		gv, err := schema.ParseGroupVersion(gvStr)
		if err != nil {
			return errors.Wrap(err, "invalid GroupVersion")
		}
		c.Watch.gvk = gv.WithKind(kind)
	}

	kongCtx.BindTo(ctx, (*context.Context)(nil))

	return nil
}

func (c *generateCmd) Run(ctx context.Context, p pterm.TextPrinter) error {
	operation, err := c.newOperation(ctx)
	if err != nil {
		return err
	}

	var out runtime.Object
	switch {
	case c.Cron != "":
		out = makeCronOperation(c.Cron, operation)

	case c.Watch.GroupVersionKind != "":
		out = makeWatchOperation(c.Watch, operation)

	default:
		out = operation
	}

	// Convert Operation to YAML format. Remove the operationTemplate.metadata
	// from cron/watch operations, since it contains an unnecessary null
	// creationTimestamp and nothing else.
	outYAML, err := yaml.Marshal(out,
		yaml.RemoveField("metadata.creationTimestamp"),
		yaml.RemoveField("spec.operationTemplate.metadata"),
		yaml.RemoveField("status"),
	)
	if err != nil {
		return errors.Wrap(err, "failed to marshal operation to yaml")
	}

	switch c.Output {
	case outputFile:
		// Determine the file path
		filePath := c.Path
		if filePath == "" {
			filePath = fmt.Sprintf("%s/operation.yaml", strings.ToLower(c.Name))
		}
		fullPath := filesystem.FullPath(c.opsFS, filePath)

		// Check if the operation already exists
		exists, err := afero.Exists(c.opsFS, filePath)
		if err != nil {
			return errors.Wrap(err, "failed to check if file exists")
		}

		// If the file exists, prompt the user for confirmation to overwrite
		if exists {
			pterm.Println() // Blank line for spacing
			confirm := pterm.DefaultInteractiveConfirm

			confirm.DefaultText = fmt.Sprintf("The Operation file '%s' already exists. Do you want to overwrite its contents?", fullPath)
			confirm.DefaultValue = false

			result, _ := confirm.Show() // Display confirmation prompt
			pterm.Println()             // Blank line for spacing

			if !result {
				return errors.New("operation cancelled by user")
			}
		}

		if err := c.opsFS.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
			return errors.Wrap(err, "failed to create directories for the specified output path")
		}

		// Write the YAML to the specified output file
		if err := afero.WriteFile(c.opsFS, filePath, outYAML, 0o644); err != nil {
			return errors.Wrap(err, "failed to write operation to file")
		}

		p.Printfln("successfully created %s and saved to %s", out.GetObjectKind().GroupVersionKind().Kind, fullPath)

	case outputYAML:
		p.Println(string(outYAML))

	case outputJSON:
		jsonData, err := yaml.YAMLToJSON(outYAML)
		if err != nil {
			return errors.Wrap(err, "failed to convert operation to JSON")
		}
		p.Println(string(jsonData))

	default:
		return errors.New("invalid output format specified")
	}

	return nil
}

// newOperation to create a new Operation.
func (c *generateCmd) newOperation(ctx context.Context) (*opsv1alpha1.Operation, error) {
	pipelineDeps, err := c.ensureFunctionDeps(ctx)
	if err != nil {
		return nil, err
	}

	operation := &opsv1alpha1.Operation{
		TypeMeta: metav1.TypeMeta{
			APIVersion: opsv1alpha1.OperationGroupVersionKind.GroupVersion().String(),
			Kind:       opsv1alpha1.OperationKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: c.Name,
		},
		Spec: opsv1alpha1.OperationSpec{
			RetryLimit: ptr.To(int64(3)),
			Mode:       opsv1alpha1.OperationModePipeline,
		},
	}

	for _, dep := range pipelineDeps {
		input, err := c.generateInput(ctx, dep)
		if err != nil {
			return nil, err
		}

		fnRepo, err := name.NewRepository(ptr.Deref(dep.Package, ""), name.StrictValidation)
		if err != nil {
			return nil, errors.Wrapf(err, "function %q has an invalid repository name", ptr.Deref(dep.Package, ""))
		}
		fnName := xpkg.ToDNSLabel(fnRepo.RepositoryStr())

		operation.Spec.Pipeline = append(operation.Spec.Pipeline, opsv1alpha1.PipelineStep{
			Step: fnName,
			FunctionRef: opsv1alpha1.FunctionReference{
				Name: fnName,
			},
			Input: input,
		})
	}

	return operation, nil
}

func (c *generateCmd) ensureFunctionDeps(ctx context.Context) ([]pkgmetav1.Dependency, error) {
	currentFns := make(map[string]pkgmetav1.Dependency)
	for _, dep := range c.proj.Spec.DependsOn {
		currentFns[ptr.Deref(dep.Package, "")] = dep
	}

	pipelineDeps := make([]pkgmetav1.Dependency, len(c.Functions))
	for i, fnXpkg := range c.Functions {
		existing, ok := currentFns[fnXpkg]
		if ok {
			pipelineDeps[i] = existing
			continue
		}

		d := pkgmetav1.Dependency{
			APIVersion: ptr.To(pkgv1.FunctionGroupVersionKind.GroupVersion().String()),
			Kind:       &pkgv1.FunctionKind,
			Package:    ptr.To(fnXpkg),
			Version:    ">=v0.0.0",
		}
		if err := c.depManager.Add(ctx, d); err != nil {
			return nil, errors.Wrapf(err, "failed to add dependency on function %q", fnXpkg)
		}
		pipelineDeps[i] = d
	}

	return pipelineDeps, nil
}

func (c *generateCmd) generateInput(ctx context.Context, dep pkgmetav1.Dependency) (*runtime.RawExtension, error) {
	pkg, err := c.depManager.GetParsedPackage(ctx, dep)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get package %q from dependency manager", ptr.Deref(dep.Package, ""))
	}

	if len(pkg.Objs) == 0 {
		// Function does not have an input type.
		return nil, nil
	}

	crd, ok := pkg.Objs[0].(*apiextensionsv1.CustomResourceDefinition)
	if !ok {
		// Unexpectedly, the function has a non-CRD object. Treat it as though
		// there's no input type.
		return nil, nil
	}

	yamlData, err := icrd.GenerateExample(*crd, false, false)
	if err != nil {
		return nil, errors.Wrap(err, "failed generating schema")
	}
	jsonData, err := json.Marshal(yamlData)
	if err != nil {
		return nil, errors.Wrap(err, "failed marshaling to JSON")
	}

	raw := &runtime.RawExtension{
		Raw: jsonData,
	}

	return raw, nil
}

func makeCronOperation(schedule string, op *opsv1alpha1.Operation) *opsv1alpha1.CronOperation {
	return &opsv1alpha1.CronOperation{
		TypeMeta: metav1.TypeMeta{
			APIVersion: opsv1alpha1.CronOperationGroupVersionKind.GroupVersion().String(),
			Kind:       opsv1alpha1.CronOperationKind,
		},
		ObjectMeta: op.ObjectMeta,
		Spec: opsv1alpha1.CronOperationSpec{
			Schedule: schedule,
			OperationTemplate: opsv1alpha1.OperationTemplate{
				Spec: op.Spec,
			},
		},
	}
}

func makeWatchOperation(watch watchSpec, op *opsv1alpha1.Operation) *opsv1alpha1.WatchOperation {
	return &opsv1alpha1.WatchOperation{
		TypeMeta: metav1.TypeMeta{
			APIVersion: opsv1alpha1.WatchOperationGroupVersionKind.GroupVersion().String(),
			Kind:       opsv1alpha1.WatchOperationKind,
		},
		ObjectMeta: op.ObjectMeta,
		Spec: opsv1alpha1.WatchOperationSpec{
			Watch: opsv1alpha1.WatchSpec{
				APIVersion:  watch.gvk.GroupVersion().String(),
				Kind:        watch.gvk.Kind,
				MatchLabels: watch.MatchLabels,
				Namespace:   watch.Namespace,
			},
			OperationTemplate: opsv1alpha1.OperationTemplate{
				Spec: op.Spec,
			},
		},
	}
}
