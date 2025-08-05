// Copyright 2025 Upbound Inc.
// All rights reserved

package test

import (
	"context"
	"embed"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/internal/kcl"
	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/pkg/apis"
	"github.com/upbound/up/pkg/apis/project/v2alpha1"
)

func (c *generateCmd) Help() string {
	return `
The 'generate' command creates tests in the specified language.

Examples:
    test generate xstoragebucket
        Creates a composition test with the default language (KCL) in the folder 'tests/test-xstoragebucket'.

    test generate xstoragebucket --language python
        Creates a composition test with Python language support in the folder 'tests/test-xstoragebucket'.

    test generate xstoragebucket --language python --e2e
        Creates a e2etest with Python language support in the folder 'tests/e2etest-xstoragebucket'.
`
}

// Embed templates for languages.
var (
	//go:embed templates/kcl/**
	kclTemplate embed.FS

	//go:embed templates/python/**
	pythonTemplate embed.FS
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
	ProjectFile string `default:"upbound.yaml"  help:"Path to project definition file." short:"f"`
	CacheDir    string `default:"~/.up/cache/"  env:"CACHE_DIR"                         help:"Directory used for caching dependency images." type:"path"`
	Language    string `default:"kcl"           enum:"kcl,python"                       help:"Language for test."                            short:"l"`
	Name        string `arg:""                  help:"Name for the new Function."       required:""`
	E2E         bool   `help:"create e2e tests" name:"e2e"`

	testFS   afero.Fs
	modelsFS afero.Fs
	projFS   afero.Fs
	fsPath   string
	testName string
	proj     *v2alpha1.Project

	m *project.DependencyManager
}

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *generateCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	kongCtx.Bind(pterm.DefaultBulletList.WithWriter(kongCtx.Stdout))
	ctx := context.Background()

	c.testName = fmt.Sprintf("test-%s", c.Name)
	if c.E2E {
		c.testName = fmt.Sprintf("e2etest-%s", c.Name)
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

	// The tests path is relative to the project directory; prepend it with
	// `/` to make it an absolute path within the project FS.
	c.fsPath = filepath.Join(
		"/",
		proj.Spec.Paths.Tests,
		c.testName,
	)

	c.testFS = afero.NewBasePathFs(
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

	return nil
}

// Run executes the test generation command.
func (c *generateCmd) Run(ctx context.Context, printer upterm.ObjectPrinter) error {
	var (
		err            error
		testSpecificFs = afero.NewBasePathFs(afero.NewOsFs(), ".")
	)

	if errs := validation.IsDNS1035Label(c.testName); len(errs) > 0 {
		return errors.Errorf("'%s' is not a valid test name. DNS-1035 constraints: %s", c.testName, strings.Join(errs, "; "))
	}

	isEmpty, err := filesystem.IsFsEmpty(c.testFS)
	if err != nil {
		pterm.Error.Println("Failed to check if the filesystem is empty:", err)
		return err
	}

	if !isEmpty {
		pterm.Println()
		confirm := pterm.DefaultInteractiveConfirm
		confirm.DefaultText = fmt.Sprintf("The folder '%s' is not empty. Do you want to overwrite its contents?", filesystem.FullPath(c.projFS, c.fsPath))
		confirm.DefaultValue = false
		result, _ := confirm.Show()
		pterm.Println()

		if !result {
			pterm.Error.Println("The operation was cancelled.")
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

	// * Generate schemas for meta apis.
	if err := apis.GenerateSchema(ctx, c.m.SchemaManager()); err != nil {
		return errors.Wrap(err, "unable to generate meta apis schemas")
	}

	switch c.Language {
	case "kcl":
		testSpecificFs, err = c.generateKCLFiles()
		if err != nil {
			return errors.Wrap(err, "failed to generate KCL test")
		}
	case "python":
		testSpecificFs, err = c.generatePythonFiles()
		if err != nil {
			return errors.Wrap(err, "failed to generate Python test")
		}
	default:
		return errors.Errorf("unsupported language: %s", c.Language)
	}

	err = upterm.WrapWithSuccessSpinner(
		"Generating Test Folder",
		upterm.CheckmarkSuccessSpinner,
		func() error {
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

// generateKCLFiles reads and processes Go template files from embed.FS.
func (c *generateCmd) generateKCLFiles() (afero.Fs, error) {
	targetFS := afero.NewMemMapFs()

	baseFolder := "compositiontest"
	if c.E2E {
		baseFolder = "e2e"
	}

	templates := template.Must(template.ParseFS(kclTemplate, fmt.Sprintf("templates/kcl/%s/**", baseFolder)))

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

	baseFolder := "compositiontest"
	if c.E2E {
		baseFolder = "e2e"
	}

	// Note that currently our python templates don't actually do any
	// templating, hence the empty template data. But we render them with the
	// same mechanism we use for other languages to maximize code reuse and
	// allow for richer templates in the future.
	templates := template.Must(template.ParseFS(pythonTemplate, fmt.Sprintf("templates/python/%s/**", baseFolder)))

	tmplData := struct{}{}

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
