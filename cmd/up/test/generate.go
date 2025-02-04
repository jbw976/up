// Copyright 2024 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package test

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/internal/xpkg/dep/cache"
	"github.com/upbound/up/internal/xpkg/dep/manager"
	"github.com/upbound/up/internal/xpkg/dep/resolver/image"
	"github.com/upbound/up/internal/xpkg/schemarunner"
	"github.com/upbound/up/internal/xpkg/workspace"
	"github.com/upbound/up/pkg/apis"
)

func (c *generateCmd) Help() string {
	return `
The 'generate' command creates tests in the specified language.

Examples:
    test generate xstoragebucket
        Creates a composition test with the default language (KCL) in the folder 'tests/test-xstoragebucket'.

    function generate xstoragebucket --language python
        Creates a composition test with Python language support in the folder 'tests/test-xstoragebucket'.

    function generate xstoragebucket --language python --e2e
        Creates a e2etest with Python language support in the folder 'tests/e2etest-xstoragebucket'.
`
}

const kclModTemplate = `[package]
name = "{{.Name}}"
version = "0.0.1"

[dependencies]
models = { path = "./model" }
`

const kclModLockTemplate = `[dependencies]
  [dependencies.model]
    name = "model"
    full_name = "models_0.0.1"
    version = "0.0.1"
`

const kclE2ETestTemplate = `{{- if .Imports }}
{{- range .Imports }}
import {{.ImportPath}} as {{.Alias}}
{{- end }}
{{- "\n" }}
{{- end }}

# _items = [
#     metav1alpha1.E2ETest{
#         spec = {
#             crossplane.autoUpgrade.channel = "Rapid"
#             defaultConditions= ["Ready"]
#             manifests= []
#             extraResources= []
#             skipDelete: False
#             timeoutSeconds: 4500
#         }
#     }
# ]
# items = _items

`

const kclCompositionTestTemplate = `{{- if .Imports }}
{{- range .Imports }}
import {{.ImportPath}} as {{.Alias}}
{{- end }}
{{- "\n" }}
{{- end }}

# _items = [
#     metav1alpha1.CompositionTest{
#         spec= {
#             assert: []
#             composition: ""
#             xr: ""
#             xrd: ""
#             context: []
#             extraResources: []
#             observedResources: []
#             timeoutSeconds: 60
#             validate: False
#         }
#     }
# ]
# items = _items

`

const pythonReqTemplate = `pydantic==2.9.2
`

const pythonMainTemplate = `from .model.io.upbound.dev.meta.e2etest import v1alpha1 as e2etest
    from .model.io.k8s.apimachinery.pkg.apis.meta import v1 as k8s
`

type kclModInfo struct {
	Name string
}

// Prepare formatted import paths for the template.
type kclImportStatement struct {
	ImportPath string
	Alias      string
}

type generateCmd struct {
	ProjectFile string `default:"upbound.yaml"                                                                           help:"Path to project definition file." short:"f"`
	CacheDir    string `default:"~/.up/cache/"                                                                           env:"CACHE_DIR"                         help:"Directory used for caching dependency images." short:"d" type:"path"`
	Language    string `default:"kcl"                                                                                    enum:"kcl,python"                       help:"Language for test."                        short:"l"`
	Name        string `arg:""                                                                                           help:"Name for the new Function."       required:""`
	E2E         bool   `help:"create e2e tests"                                                                          name:"e2e"`

	testFS   afero.Fs
	modelsFS afero.Fs
	projFS   afero.Fs
	fsPath   string
	testName string

	m            *manager.Manager
	ws           *workspace.Workspace
	schemaRunner schemarunner.SchemaRunner

	quiet config.QuietFlag
}

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *generateCmd) AfterApply(kongCtx *kong.Context, quiet config.QuietFlag) error {
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
	c.modelsFS = afero.NewBasePathFs(afero.NewOsFs(), filepath.Join(projDirPath, ".up"))

	// The location of the co position defines the root of the function.
	proj, err := project.Parse(c.projFS, c.ProjectFile)
	if err != nil {
		return err
	}
	proj.Default()

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

	fs := afero.NewOsFs()

	cache, err := cache.NewLocal(c.CacheDir, cache.WithFS(fs))
	if err != nil {
		return err
	}

	r := image.NewResolver()

	m, err := manager.New(
		manager.WithCacheModels(c.modelsFS),
		manager.WithCache(cache),
		manager.WithResolver(r),
	)
	if err != nil {
		return err
	}

	c.m = m

	ws, err := workspace.New("/",
		workspace.WithFS(c.projFS),
		// The user doesn't care about workspace warnings during test generate.
		workspace.WithPrinter(&pterm.BasicTextPrinter{Writer: io.Discard}),
		workspace.WithPermissiveParser(),
	)
	if err != nil {
		return err
	}
	c.ws = ws

	if err := ws.Parse(ctx); err != nil {
		return err
	}
	c.schemaRunner = schemarunner.RealSchemaRunner{}

	kongCtx.BindTo(ctx, (*context.Context)(nil))

	c.quiet = quiet

	return nil
}

func (c *generateCmd) Run(ctx context.Context) error {
	var (
		err            error
		testSpecificFs = afero.NewBasePathFs(afero.NewOsFs(), ".")
	)
	pterm.EnableStyling()

	if errs := validation.IsDNS1035Label(c.testName); len(errs) > 0 {
		return errors.Errorf("'%s' is not a valid test name. DNS-1035 constraints: %s", c.testName, strings.Join(errs, "; "))
	}

	isEmpty, err := filesystem.IsFsEmpty(c.testFS)
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
			pterm.Error.Println("The operation was cancelled. The test folder must be empty to proceed with the generation.")
			return errors.New("operation cancelled by user")
		}
	}

	err = upterm.WrapWithSuccessSpinner("Checking dependencies", upterm.CheckmarkSuccessSpinner, func() error {
		deps, _ := c.ws.View().Meta().DependsOn()

		// Check all dependencies in the cache
		for _, dep := range deps {
			_, _, err := c.m.AddAll(ctx, dep)
			if err != nil {
				return errors.Wrapf(err, "failed to check dependencies for %v", dep)
			}
		}
		return nil
	}, c.quiet)
	if err != nil {
		return err
	}

	// * Generate schemas for meta apis
	if err = apis.GenerateSchema(ctx, c.m, c.schemaRunner); err != nil {
		return errors.Wrap(err, "unable to generate meta apis schemas")
	}

	switch c.Language {
	case "kcl":
		testSpecificFs, err = c.generateKCLFiles()
		if err != nil {
			return errors.Wrap(err, "failed to handle kcl")
		}
	case "python":
		testSpecificFs, err = generatePythonFiles()
		if err != nil {
			return errors.Wrap(err, "failed to handle python")
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

			return nil
		}, c.quiet)
	if err != nil {
		return err
	}

	pterm.Printfln("successfully created Test and saved to %s", filesystem.FullPath(c.projFS, c.fsPath))
	return nil
}

func (c *generateCmd) generateKCLFiles() (afero.Fs, error) {
	targetFS := afero.NewMemMapFs()

	kclModInfo := kclModInfo{
		Name: c.testName,
	}

	kclModPath := "kcl.mod"
	file, err := targetFS.Create(filepath.Clean(kclModPath))
	if err != nil {
		return nil, errors.Wrapf(err, "error creating file: %v", kclModPath)
	}

	tmpl := template.Must(template.New("toml").Parse(kclModTemplate))
	if err := tmpl.Execute(file, kclModInfo); err != nil {
		return nil, errors.Wrapf(err, "Error writing template to file: %v", kclModPath)
	}

	kclModLockPath := "kcl.mod.lock"
	if exists, err := afero.Exists(targetFS, kclModLockPath); err != nil {
		return nil, errors.Wrapf(err, "error checking file existence: %v", kclModLockPath)
	} else if !exists {
		file, err := targetFS.Create(filepath.Clean(kclModLockPath))
		if err != nil {
			return nil, errors.Wrapf(err, "error creating file: %v", kclModLockPath)
		}

		_, err = file.WriteString(kclModLockTemplate)
		if err != nil {
			return nil, errors.Wrapf(err, "error writing to file: %v", kclModLockPath)
		}
	}
	mainPath := "main.k"
	file, err = targetFS.Create(filepath.Clean(mainPath))
	if err != nil {
		return nil, errors.Wrapf(err, "error creating file: %v", mainPath)
	}
	foundFolders, _ := filesystem.FindNestedFoldersWithPattern(c.modelsFS, "kcl/models", "*.k")

	importStatements := make([]kclImportStatement, 0, len(foundFolders))
	for _, folder := range foundFolders {
		importPath, alias := formatKclImportPath(folder)
		importStatements = append(importStatements, kclImportStatement{
			ImportPath: importPath,
			Alias:      alias,
		})
	}
	mainTemplateData := struct {
		Imports []kclImportStatement
	}{
		Imports: importStatements,
	}
	mainTmpl := template.Must(template.New("kcl").Parse(kclCompositionTestTemplate))
	if c.E2E {
		mainTmpl = template.Must(template.New("kcl").Parse(kclE2ETestTemplate))
	}

	if err := mainTmpl.Execute(file, mainTemplateData); err != nil {
		return nil, errors.Wrapf(err, "Error writing KCL template to file: %v", mainPath)
	}

	return targetFS, nil
}

func generatePythonFiles() (afero.Fs, error) {
	targetFS := afero.NewMemMapFs()

	mainPath := "main.py"
	pythonReqPath := "requirements.txt"

	if exists, err := afero.Exists(targetFS, pythonReqPath); err != nil {
		return nil, errors.Wrapf(err, "error checking file existence: %v", pythonReqPath)
	} else if !exists {
		file, err := targetFS.Create(filepath.Clean(pythonReqPath))
		if err != nil {
			return nil, errors.Wrapf(err, "error creating file: %v", pythonReqPath)
		}

		_, err = file.WriteString(pythonReqTemplate)
		if err != nil {
			return nil, errors.Wrapf(err, "error writing to file: %v", pythonReqPath)
		}
	}

	if exists, err := afero.Exists(targetFS, mainPath); err != nil {
		return nil, errors.Wrapf(err, "error checking file existence: %v", mainPath)
	} else if !exists {
		file, err := targetFS.Create(filepath.Clean(mainPath))
		if err != nil {
			return nil, errors.Wrapf(err, "error creating file: %v", mainPath)
		}

		_, err = file.WriteString(pythonMainTemplate)
		if err != nil {
			return nil, errors.Wrapf(err, "error writing to file: %v", mainPath)
		}
	}

	return targetFS, nil
}

// Helper function to convert kcl paths to the desired import format.
func formatKclImportPath(path string) (string, string) {
	// Find the position of "models" in the path and keep only the part after it
	modelsIndex := strings.Index(path, "models")
	if modelsIndex == -1 {
		return "", ""
	}

	// Trim everything before "models" and replace slashes with dots
	importPath := strings.ReplaceAll(path[modelsIndex:], "/", ".")

	// Extract alias using the last two components of the path
	parts := strings.Split(importPath, ".")
	alias := parts[len(parts)-2] + parts[len(parts)-1] // e.g., redshiftv1beta1

	return importPath, alias
}
