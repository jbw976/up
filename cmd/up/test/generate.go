// Copyright 2025 Upbound Inc.
// All rights reserved

package test

import (
	"archive/tar"
	"bytes"
	"context"
	"embed"
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	"github.com/spf13/afero"
	"github.com/spf13/afero/tarfs"
	"golang.org/x/mod/module"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/internal/kcl"
	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/pkg/apis"
	"github.com/upbound/up/pkg/apis/project/v2alpha1"
)

//go:embed help/generate.md
var generateHelp string

func (c *generateCmd) Help() string {
	return generateHelp
}

// Embed templates for languages.
var (
	//go:embed templates/kcl/**
	kclTemplate embed.FS

	//go:embed templates/python/**
	pythonTemplate embed.FS

	// The go templates contain go.mod files, so we can't embed them as an
	// embed.FS. Instead we have to embed them as tar archives and extract them in
	// code.
	//go:embed templates/go/compositiontest.tar
	goCompositionTestTemplate []byte

	//go:embed templates/go/e2e.tar
	goE2ETestTemplate []byte

	//go:embed templates/go/operationtest.tar
	goOperationTestTemplate []byte

	//go:embed templates/go-templating/**
	goTemplatingTemplate embed.FS
)

// Template data structure for dynamic rendering.
type kclTemplateData struct {
	ModName string
	Imports []kclImportStatement
}

type kclImportStatement struct {
	ImportPath string
	Alias      string
}
type generateCmd struct {
	ProjectFile string `default:"upbound.yaml"        help:"Path to project definition file." short:"f"`
	CacheDir    string `default:"~/.up/cache/"        env:"CACHE_DIR"                         help:"Directory used for caching dependency images." type:"path"`
	Language    string `default:"kcl"                 enum:"go,go-templating,kcl,python"      help:"Language for test."                            short:"l"   telemetry:"true"`
	Name        string `arg:""                        help:"Name for the new Function."       required:""`
	E2E         bool   `help:"create e2e tests"       name:"e2e"`
	Operation   bool   `help:"create operation tests" name:"operation"`

	testFS             afero.Fs
	modelsFS           afero.Fs
	projFS             afero.Fs
	fsPath             string
	testName           string
	templateBaseFolder string
	proj               *v2alpha1.Project

	m *project.DependencyManager
}

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *generateCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	kongCtx.Bind(pterm.DefaultBulletList.WithWriter(kongCtx.Stdout))
	ctx := context.Background()

	// Set test name and template base folder based on test type
	c.testName = fmt.Sprintf("test-%s", c.Name)
	c.templateBaseFolder = "compositiontest"

	if c.E2E {
		c.testName = fmt.Sprintf("e2etest-%s", c.Name)
		c.templateBaseFolder = "e2e"
	}
	if c.Operation {
		c.testName = fmt.Sprintf("operationtest-%s", c.Name)
		c.templateBaseFolder = "operationtest"
	}

	// Read the project file.
	projFilePath, err := filepath.Abs(c.ProjectFile)
	if err != nil {
		return err
	}
	// The location of the project file defines the root of the project.
	projDirPath := filepath.Dir(projFilePath)
	c.projFS = afero.NewBasePathFs(afero.NewOsFs(), projDirPath)
	c.modelsFS = afero.NewBasePathFs(c.projFS, ".up")

	// The location of the co position defines the root of the function.
	proj, err := project.Parse(c.projFS, filepath.Base(c.ProjectFile))
	if err != nil {
		return err
	}
	proj.Default()
	c.proj = proj

	// The tests path is relative to the project directory
	// Don't use leading `/` on Windows as BasePathFs treats it as absolute
	c.fsPath = path.Join(
		proj.Spec.Paths.Tests,
		c.testName,
	)

	c.testFS = afero.NewBasePathFs(
		afero.NewOsFs(), filepath.Join(projDirPath, c.fsPath),
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

	return nil
}

// Run executes the test generation command.
func (c *generateCmd) Run(ctx context.Context, printer upterm.ObjectPrinter) error {
	if errs := validation.IsDNS1035Label(c.testName); len(errs) > 0 {
		return errors.Errorf("'%s' is not a valid test name. DNS-1035 constraints: %s", c.testName, strings.Join(errs, "; "))
	}

	isEmpty, err := filesystem.IsFsEmpty(c.testFS)
	if err != nil {
		pterm.Error.Println("Failed to check if the filesystem is empty:", err)
		return err
	}

	if !isEmpty {
		result, _ := upterm.Confirm(fmt.Sprintf("The folder '%s' is not empty. Do you want to overwrite its contents?", filesystem.FullPath(c.projFS, c.fsPath)), false)

		if !result {
			pterm.Error.Println("The operation was cancelled.")
			return errors.New("operation cancelled by user")
		}
	}

	err = upterm.WrapWithSuccessSpinner("Checking dependencies", func() error {
		err := c.m.AddAll(ctx, c.proj.Spec.DependsOn...)
		if err != nil {
			return err
		}
		return c.m.AddAllAPIDependencies(ctx, c.proj.Spec.APIDependencies)
	}, printer)
	if err != nil {
		return err
	}

	// * Generate schemas for meta apis.
	if err := apis.GenerateSchema(ctx, c.m.SchemaManager()); err != nil {
		return errors.Wrap(err, "unable to generate meta apis schemas")
	}

	err = upterm.WrapWithSuccessSpinner(
		"Generating Test Folder",
		func() error {
			testSpecificFs, err := c.generateFiles()
			if err != nil {
				return err
			}

			if err := filesystem.CopyFilesBetweenFs(testSpecificFs, c.testFS); err != nil {
				return errors.Wrap(err, "failed to copy files to test target")
			}

			if needsModelsSymlink(c.Language) {
				modelsPath := ".up/" + c.Language + "/models"

				testFS, ok := c.testFS.(*afero.BasePathFs)
				if !ok {
					return errors.Errorf("unexpected filesystem type %T for tests", testFS)
				}
				projFS, ok := c.projFS.(*afero.BasePathFs)
				if !ok {
					return errors.Errorf("unexpected filesystem type %T for project", projFS)
				}
				if err := filesystem.CreateSymlink(testFS, "model", projFS, modelsPath); err != nil {
					return errors.Wrapf(err, "error creating models symlink")
				}
			}
			return nil
		}, printer)
	if err != nil {
		return err
	}

	pterm.Printfln("Successfully created Test and saved to %s", filesystem.FullPath(c.projFS, c.fsPath))
	return nil
}

func (c *generateCmd) generateFiles() (afero.Fs, error) {
	switch c.Language {
	case "go":
		return c.generateGoFiles()

	case "kcl":
		return c.generateKCLFiles()

	case "python":
		return c.generatePythonFiles()

	case "go-templating":
		return c.generateGoTemplatingFiles()

	default:
		return nil, errors.Errorf("unsupported language: %s", c.Language)
	}
}

// generateKCLFiles reads and processes Go template files from embed.FS.
func (c *generateCmd) generateKCLFiles() (afero.Fs, error) {
	targetFS := afero.NewMemMapFs()

	templates := template.Must(template.ParseFS(kclTemplate, fmt.Sprintf("templates/kcl/%s/**", c.templateBaseFolder)))

	foundFolders, _ := filesystem.FindNestedFoldersWithPattern(c.modelsFS, "kcl/models", "*.k")
	importStatements := make([]kclImportStatement, 0, len(foundFolders))

	// Track existing aliases to prevent duplicates
	existingAliases := make(map[string]bool)

	for _, folder := range foundFolders {
		importPath, alias := kcl.FormatKclImportPath(folder, existingAliases)
		importStatements = append(importStatements, kclImportStatement{
			ImportPath: importPath,
			Alias:      alias,
		})
	}

	tmplData := kclTemplateData{
		ModName: c.testName,
		Imports: importStatements,
	}

	if err := renderTemplates(targetFS, templates, tmplData); err != nil {
		return nil, err
	}

	return targetFS, nil
}

// generatePythonFiles reads and processes Go template files from embed.FS.
func (c *generateCmd) generatePythonFiles() (afero.Fs, error) {
	targetFS := afero.NewMemMapFs()

	// Note that currently our python templates don't actually do any
	// templating, hence the empty template data. But we render them with the
	// same mechanism we use for other languages to maximize code reuse and
	// allow for richer templates in the future.
	templates := template.Must(template.ParseFS(pythonTemplate, fmt.Sprintf("templates/python/%s/**", c.templateBaseFolder)))

	tmplData := struct{}{}

	if err := renderTemplates(targetFS, templates, tmplData); err != nil {
		return nil, err
	}

	return targetFS, nil
}

type goTemplateData struct {
	ModulePath    string
	ModelsVersion string
	ModelsReplace string
}

// generateGoFiles reads and processes Go template files from tar archives.
func (c *generateCmd) generateGoFiles() (afero.Fs, error) {
	targetFS := afero.NewMemMapFs()

	// Select the appropriate template based on test type
	var templateBytes []byte
	switch c.templateBaseFolder {
	case "compositiontest":
		templateBytes = goCompositionTestTemplate
	case "e2e":
		templateBytes = goE2ETestTemplate
	case "operationtest":
		templateBytes = goOperationTestTemplate
	default:
		return nil, errors.Errorf("unsupported template type: %s", c.templateBaseFolder)
	}

	tr := tar.NewReader(bytes.NewReader(templateBytes))
	templateFS := afero.NewIOFS(tarfs.New(tr))

	// Try to construct a nice import path based on the project's "source"
	// field, which the user should fill in with their git repository path
	// (possibly with https:// prefixed if it's a GH repository). If that's not
	// valid, construct an example path we know is valid. The import path
	// doesn't actually matter to the builder aside from being valid.
	source := strings.TrimPrefix(c.proj.Spec.Source, "https://")
	goModPath := path.Join(source, "tests", c.testName)
	if module.CheckPath(goModPath) != nil {
		goModPath = "project.example.com/tests/" + c.testName
	}

	// Figure out where the models directory will be relative to the test
	// directory so we can generate a go mod replace for it.
	testDir := filepath.Join("/", c.proj.Spec.Paths.Tests, "test")
	relRoot, err := filepath.Rel(testDir, "/")
	if err != nil {
		return nil, errors.Wrap(err, "cannot determine path to models directory")
	}

	templates := template.Must(template.ParseFS(templateFS, "*"))
	tmplData := goTemplateData{
		ModulePath:    goModPath,
		ModelsVersion: "v0.0.0",
		ModelsReplace: filepath.Join(relRoot, ".up", "go", "models"),
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

	modelPath, err := filepath.Rel(c.fsPath, ".up/json/models/test.schema.json")
	if err != nil {
		return nil, errors.Wrap(err, "cannot determine model path")
	}

	templates := template.Must(template.ParseFS(goTemplatingTemplate, fmt.Sprintf("templates/go-templating/%s/**", c.templateBaseFolder)))
	tmplData := goTemplatingTemplateData{
		ModelIndexPath: modelPath,
	}

	if err := renderTemplates(targetFS, templates, tmplData); err != nil {
		return nil, err
	}

	return targetFS, nil
}

// renderTemplates executes.
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

// needsModelsSymlink determines if a symlink is needed.
func needsModelsSymlink(language string) bool {
	switch language {
	case "kcl", "python":
		return true
	case "go":
		return false
	default:
		return false
	}
}
