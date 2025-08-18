// Copyright 2025 Upbound Inc.
// All rights reserved

package function

import (
	"archive/tar"
	"bytes"
	"context"
	"embed"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/alecthomas/kong"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/pterm/pterm"
	"github.com/spf13/afero"
	"github.com/spf13/afero/tarfs"
	"golang.org/x/mod/module"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/internal/kcl"
	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/internal/xpkg"
	"github.com/upbound/up/internal/yaml"
	"github.com/upbound/up/pkg/apis/project/v2alpha1"
)

//go:embed help/generate.md
var generateHelp string

func (c *generateCmd) Help() string {
	return generateHelp
}

var (
	//go:embed templates/kcl/**
	kclTemplate embed.FS
	//go:embed templates/python/**
	pythonTemplate embed.FS
	//go:embed templates/go-templating/**
	goTemplatingTemplate embed.FS

	// The go template contains a go.mod, so we can't embed it as an
	// embed.FS. Instead we have to embed it as a tar archive and extract it in
	// code.
	//go:embed templates/go.tar
	goTemplate []byte
)

type generateCmd struct {
	Name         string `arg:"" help:"Name for the new Function."                                         required:""`
	PipelinePath string `arg:"" help:"Path to a composition or operation that will use the new function." optional:""`

	ProjectFile string `default:"upbound.yaml"                                                                           help:"Path to project definition file." short:"f"`
	Repository  string `help:"Repository for the built package. Overrides the repository specified in the project file." optional:""`
	CacheDir    string `default:"~/.up/cache/"                                                                           env:"CACHE_DIR"                         help:"Directory used for caching dependency images." type:"path"`
	Language    string `default:"go-templating"                                                                          enum:"go,go-templating,kcl,python"      help:"Language for function."                        short:"l"   telemetry:"true"`

	functionFS        afero.Fs
	modelsFS          afero.Fs
	projFS            afero.Fs
	projectRepository string
	fsPath            string
	proj              *v2alpha1.Project

	m *project.DependencyManager

	quiet config.QuietFlag
}

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *generateCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context, quiet config.QuietFlag) error {
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
	c.modelsFS = afero.NewBasePathFs(afero.NewOsFs(), filepath.Join(projDirPath, ".up"))

	// The location of the co position defines the root of the function.
	proj, err := project.Parse(c.projFS, filepath.Base(c.ProjectFile))
	if err != nil {
		return err
	}
	proj.Default()
	c.proj = proj

	// The functions path is relative to the project directory; prepend it with
	// `/` to make it an absolute path within the project FS.
	c.fsPath = filepath.Join(
		"/",
		proj.Spec.Paths.Functions,
		c.Name,
	)

	c.projectRepository = proj.Spec.Repository
	c.functionFS = afero.NewBasePathFs(
		c.projFS, c.fsPath,
	)

	cchFS := afero.NewBasePathFs(afero.NewOsFs(), c.CacheDir)
	m, err := project.NewDependencyManager(upCtx, c.proj, c.projFS,
		project.WithCacheFS(cchFS),
	)
	if err != nil {
		return err
	}

	c.m = m

	kongCtx.BindTo(ctx, (*context.Context)(nil))

	c.quiet = quiet
	return nil
}

func (c *generateCmd) Run(ctx context.Context, printer upterm.ObjectPrinter) error { //nolint:gocognit // TODO: refactor
	var (
		err                error
		functionSpecificFs afero.Fs
	)

	if errs := validation.IsDNS1035Label(c.Name); len(errs) > 0 {
		return errors.Errorf("'%s' is not a valid function name. DNS-1035 constraints: %s", c.Name, strings.Join(errs, "; "))
	}

	if c.PipelinePath != "" {
		exists, _ := afero.Exists(c.projFS, c.PipelinePath)
		if !exists {
			return errors.Errorf("file %q does not exist", c.PipelinePath)
		}
	}

	isEmpty, err := filesystem.IsFsEmpty(c.functionFS)
	if err != nil {
		pterm.Error.Println("Failed to check if the filesystem is empty:", err)
		return err
	}

	if !isEmpty {
		// Prompt the user for confirmation to overwrite
		pterm.Println() // Blank line
		confirm := pterm.DefaultInteractiveConfirm
		confirm.DefaultText = fmt.Sprintf("The folder '%s' is not empty. Do you want to overwrite its contents?", filesystem.FullPath(c.projFS, c.fsPath))
		confirm.DefaultValue = false
		result, _ := confirm.Show()
		pterm.Println() // Blank line

		if !result {
			pterm.Error.Println("The operation was cancelled. The function folder must be empty to proceed with the generation.")
			return errors.New("operation cancelled by user")
		}
	}

	err = upterm.WrapWithSuccessSpinner("Checking dependencies", upterm.CheckmarkSuccessSpinner, func() error {
		err := c.m.AddAll(ctx, c.proj.Spec.DependsOn...)
		if err != nil {
			return err
		}
		return c.m.AddAllAPIDependencies(ctx, c.proj.Spec.APIDependencies)
	}, printer)
	if err != nil {
		return err
	}

	switch c.Language {
	case "kcl":
		functionSpecificFs, err = c.generateKCLFiles()
		if err != nil {
			return errors.Wrap(err, "failed to handle kcl")
		}
	case "python":
		functionSpecificFs, err = generatePythonFiles()
		if err != nil {
			return errors.Wrap(err, "failed to handle python")
		}
	case "go":
		functionSpecificFs, err = c.generateGoFiles()
		if err != nil {
			return errors.Wrap(err, "failed to handle go")
		}
	case "go-templating":
		functionSpecificFs, err = c.generateGoTemplatingFiles()
		if err != nil {
			return errors.Wrap(err, "failed to handle go-templating")
		}
	default:
		return errors.Errorf("unsupported language: %s", c.Language)
	}

	err = upterm.WrapWithSuccessSpinner(
		"Generating Function Folder",
		upterm.CheckmarkSuccessSpinner,
		func() error {
			if err := filesystem.CopyFilesBetweenFs(functionSpecificFs, c.functionFS); err != nil {
				return errors.Wrap(err, "failed to copy files to function target")
			}

			if !needsModelsSymlink(c.Language) {
				return nil
			}

			modelsPath := filepath.Join(".up", c.Language, "models")

			functionFS, ok := c.functionFS.(*afero.BasePathFs)
			if !ok {
				return errors.Errorf("unexpected filesystem type %T for functions", functionFS)
			}
			projFS, ok := c.projFS.(*afero.BasePathFs)
			if !ok {
				return errors.Errorf("unexpected filesystem type %T for project", projFS)
			}
			if err := filesystem.CreateSymlink(functionFS, "model", projFS, modelsPath); err != nil {
				return errors.Wrapf(err, "error creating models symlink")
			}

			return nil
		}, printer)
	if err != nil {
		return err
	}

	if c.PipelinePath != "" {
		err = upterm.WrapWithSuccessSpinner(
			"Adding Pipeline Step",
			upterm.CheckmarkSuccessSpinner,
			func() error {
				pipe, err := c.readAndUnmarshalPipeline()
				if err != nil {
					return errors.Wrapf(err, "failed to read pipeline")
				}

				if err := c.addPipelineStep(pipe); err != nil {
					return errors.Wrap(err, "failed to add pipeline step")
				}

				y, err := yaml.Marshal(pipe,
					yaml.RemoveField("spec.operationTemplate.metadata"),
					yaml.RemoveField("metadata.creationTimestamp"),
					yaml.RemoveField("status"),
				)
				if err != nil {
					return errors.Wrapf(err, "failed to marshal pipeline to yaml")
				}

				if err = afero.WriteFile(c.projFS, c.PipelinePath, y, 0o644); err != nil {
					return errors.Wrapf(err, "failed to write pipeline to file")
				}

				return nil
			},
			printer,
		)
		if err != nil {
			return err
		}
	}

	pterm.Printfln("successfully created Function and saved to %s", filesystem.FullPath(c.projFS, c.fsPath))
	return nil
}

func needsModelsSymlink(language string) bool {
	switch language {
	case "kcl", "python":
		return true
	case "go":
		// Go references modules via replace directives in go.mod rather than
		// via a symlink.
		return false
	case "go-templating":
		// go-templating references schemas by relative path in the
		// modeline. Models are required only at dev time and must not be in the
		// final function image.
		return false
	default:
		return false
	}
}

type kclTemplateData struct {
	ModName string
	Imports []kclImportStatement
}

type kclImportStatement struct {
	ImportPath string
	Alias      string
}

func (c *generateCmd) generateKCLFiles() (afero.Fs, error) {
	targetFS := afero.NewMemMapFs()

	templates := template.Must(template.ParseFS(kclTemplate, "templates/kcl/*"))

	foundFolders, _ := filesystem.FindNestedFoldersWithPattern(c.modelsFS, "kcl/models", "*.k")

	// Track existing aliases to prevent duplicates
	existingAliases := make(map[string]bool)

	importStatements := make([]kclImportStatement, 0, len(foundFolders))
	for _, folder := range foundFolders {
		importPath, alias := kcl.FormatKclImportPath(folder, existingAliases)
		importStatements = append(importStatements, kclImportStatement{
			ImportPath: importPath,
			Alias:      alias,
		})
	}
	tmplData := kclTemplateData{
		ModName: c.Name,
		Imports: importStatements,
	}

	if err := renderTemplates(targetFS, templates, tmplData); err != nil {
		return nil, err
	}

	return targetFS, nil
}

func renderTemplates(targetFS afero.Fs, templates *template.Template, data any) error {
	for _, tmpl := range templates.Templates() {
		fname := tmpl.Name()
		file, err := targetFS.Create(filepath.Clean(fname))
		if err != nil {
			return errors.Wrapf(err, "error creating file %v", fname)
		}
		if err := tmpl.Execute(file, data); err != nil {
			return errors.Wrapf(err, "error writing template to file %v", fname)
		}
		if err := file.Close(); err != nil {
			return errors.Wrapf(err, "error closing file %v", fname)
		}
	}

	return nil
}

type pythonTemplateData struct{}

func generatePythonFiles() (afero.Fs, error) {
	targetFS := afero.NewMemMapFs()

	// Note that currently our python templates don't actually do any
	// templating, hence the empty template data. But we render them with the
	// same mechanism we use for other languages to maximize code reuse and
	// allow for richer templates in the future.
	templates := template.Must(template.ParseFS(pythonTemplate, "templates/python/**"))
	tmplData := pythonTemplateData{}

	if err := renderTemplates(targetFS, templates, tmplData); err != nil {
		return nil, err
	}

	return targetFS, nil
}

type goTemplateData struct {
	ModulePath string
	Imports    []goImport
}

type goImport struct {
	Module  string
	Version string
	Replace string
}

func (c *generateCmd) generateGoFiles() (afero.Fs, error) {
	targetFS := afero.NewMemMapFs()

	tr := tar.NewReader(bytes.NewReader(goTemplate))
	templateFS := afero.NewIOFS(tarfs.New(tr))

	// Try to construct a nice import path based on the project's "source"
	// field, which the user should fill in with their git repository path
	// (possibly with https:// prefixed if it's a GH repository). If that's not
	// valid, construct an example path we know is valid. The import path
	// doesn't actually matter to the builder aside from being valid.
	source := strings.TrimPrefix(c.proj.Spec.Source, "https://")
	goModPath := path.Join(source, "functions", c.Name)
	if module.CheckPath(goModPath) != nil {
		goModPath = "project.example.com/functions/" + c.Name
	}

	// Figure out where the models directory will be relative to the function
	// directory so we can generate a go mod replace for it.
	fnDir := filepath.Join("/", c.proj.Spec.Paths.Functions, "fn")
	relRoot, err := filepath.Rel(fnDir, "/")
	if err != nil {
		return nil, errors.Wrap(err, "cannot determine path to models directory")
	}

	templates := template.Must(template.ParseFS(templateFS, "*"))
	tmplData := goTemplateData{
		ModulePath: goModPath,
		Imports: []goImport{{
			Module:  "dev.upbound.io/models",
			Version: "v0.0.0",
			Replace: filepath.Join(relRoot, ".up", "go", "models"),
		}},
	}

	if err := renderTemplates(targetFS, templates, tmplData); err != nil {
		return nil, err
	}

	return targetFS, nil
}

type goTemplatingTemplateData struct {
	ModelIndexPath string
}

func (c *generateCmd) generateGoTemplatingFiles() (afero.Fs, error) {
	targetFS := afero.NewMemMapFs()

	modelPath, err := filepath.Rel(c.fsPath, "/.up/json/models/index.schema.json")
	if err != nil {
		return nil, errors.Wrap(err, "cannot determine model path")
	}

	templates := template.Must(template.ParseFS(goTemplatingTemplate, "templates/go-templating/**"))
	tmplData := goTemplatingTemplateData{
		ModelIndexPath: modelPath,
	}

	if err := renderTemplates(targetFS, templates, tmplData); err != nil {
		return nil, err
	}

	return targetFS, nil
}

func (c *generateCmd) addPipelineStep(pipe pipeline) error {
	fnRepoStr := fmt.Sprintf("%s_%s", c.projectRepository, c.Name)
	fnRepo, err := name.NewRepository(fnRepoStr, name.StrictValidation)
	if err != nil {
		return errors.Wrapf(err, "error unable to parse the function repo")
	}

	pipe.addStep(c.Name, xpkg.ToDNSLabel(fnRepo.RepositoryStr()))

	return nil
}

func (c *generateCmd) readAndUnmarshalPipeline() (pipeline, error) {
	file, err := c.projFS.Open(c.PipelinePath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open pipeline file")
	}

	compRaw, err := io.ReadAll(file)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read pipeline file")
	}

	var u unstructured.Unstructured
	err = yaml.Unmarshal(compRaw, &u)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal pipeline")
	}

	return convertToPipeline(&u)
}
