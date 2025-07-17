// Copyright 2025 Upbound Inc.
// All rights reserved

package ai

import (
	"context"
	"embed"
	"io/fs"
	"os/user"
	"path/filepath"
	"text/template"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/pkg/apis/project/v1alpha1"
)

func (c *rulesCmd) Help() string {
	return `
The 'generate' command creates an embedded function in the specified language.

Examples:
    project ai rules --gemini
        Creates a GEMINI.md and places a settings.json under the .gemini directory.'.
`
}

var (
	//go:embed all:templates/claude
	claudeTemplate embed.FS
	//go:embed all:templates/codex
	codexTemplate embed.FS
	//go:embed all:templates/gemini
	geminiTemplate embed.FS

	claudeRoot = "templates/claude"
	codexRoot  = "templates/codex"
	geminiRoot = "templates/gemini"
)

type rulesCmd struct {
	ProjectFile string `default:"upbound.yaml" help:"Path to project definition file." short:"f"`

	Gemini bool `default:"false" group:"Tooling Provider Flags:" help:"Generate gemini CLI configurations."`
	Claude bool `default:"false" group:"Tooling Provider Flags:" help:"Generate claude code CLI configurations."`
	Codex  bool `default:"false" group:"Tooling Provider Flags:" help:"Generate codex CLI configurations."`

	Flags upbound.Flags `embed:""`

	projFS afero.Fs
	proj   *v1alpha1.Project

	user *user.User
}

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *rulesCmd) AfterApply(kongCtx *kong.Context, quiet config.QuietFlag) error {
	kongCtx.Bind(pterm.DefaultBulletList.WithWriter(kongCtx.Stdout))
	ctx := context.Background()

	upCtx, err := upbound.NewFromFlags(c.Flags)
	if err != nil {
		return err
	}
	upCtx.SetupLogging()
	kongCtx.Bind(upCtx)

	c.user, err = user.Current()
	if err != nil {
		return errors.Wrap(err, "error retrieving current user")
	}

	// Read the project file.
	projFilePath, err := filepath.Abs(c.ProjectFile)
	if err != nil {
		return err
	}
	// The location of the project file defines the root of the project.
	projDirPath := filepath.Dir(projFilePath)
	c.projFS = afero.NewBasePathFs(afero.NewOsFs(), projDirPath)

	// parse the project
	proj, err := project.Parse(c.projFS, filepath.Base(c.ProjectFile))
	if err != nil {
		return err
	}
	proj.Default()
	c.proj = proj

	kongCtx.BindTo(ctx, (*context.Context)(nil))

	return nil
}

func (c *rulesCmd) Run(ctx context.Context, printer upterm.ObjectPrinter) (err error) {
	cfgFS := []afero.Fs{}

	switch {
	case c.Gemini:
		fs, err := c.generateGeminiTemplates()
		if err != nil {
			return errors.Wrap(err, "failed to handle gemini templates")
		}
		cfgFS = append(cfgFS, fs)
	case c.Claude:
		fs, err := c.generateClaudeTemplates()
		if err != nil {
			return errors.Wrap(err, "failed to handle claude templates")
		}
		cfgFS = append(cfgFS, fs)
	case c.Codex:
		fs, err := c.generateCodexTemplates()
		if err != nil {
			return errors.Wrap(err, "failed to handle codex templates")
		}
		cfgFS = append(cfgFS, fs)
	}

	err = upterm.WrapWithSuccessSpinner(
		"Generating AI Rules Configurations",
		upterm.CheckmarkSuccessSpinner,
		func() error {
			for _, fs := range cfgFS {
				if err := filesystem.CopyFilesBetweenFs(fs, c.projFS); err != nil {
					return errors.Wrap(err, "failed to copy files to function target")
				}

				projFS, ok := c.projFS.(*afero.BasePathFs)
				if !ok {
					return errors.Errorf("unexpected filesystem type %T for project", projFS)
				}
			}
			return nil
		}, printer)
	if err != nil {
		return err
	}

	pterm.Printfln("successfully created configurations and saved to %s", filesystem.FullPath(c.projFS, ""))
	return nil
}

func renderTemplates(targetFS afero.Fs, tmpls map[string]*template.Template, data any) error {
	for path, tmpl := range tmpls {
		file, err := targetFS.Create(filepath.Clean(path))
		if err != nil {
			return errors.Wrapf(err, "error creating file %v", path)
		}
		if err := tmpl.Execute(file, data); err != nil {
			return errors.Wrapf(err, "error writing template to file %v", path)
		}
		if err := file.Close(); err != nil {
			return errors.Wrapf(err, "error closing file %v", path)
		}
	}
	return nil
}

type templateData struct {
	ProjectName string
	// Path for .up/config.json
	UpConfigDir string
}

func (c *rulesCmd) generateGeminiTemplates() (afero.Fs, error) {
	targetFS := afero.NewMemMapFs()
	cd, err := config.GetUpConfigDir()
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve up config")
	}

	tmpls := ParseTemplates(geminiTemplate, geminiRoot)

	tmplData := templateData{
		ProjectName: c.proj.Name,
		UpConfigDir: cd,
	}

	if err := renderTemplates(targetFS, tmpls, tmplData); err != nil {
		return nil, err
	}

	return targetFS, nil
}

func (c *rulesCmd) generateClaudeTemplates() (afero.Fs, error) {
	targetFS := afero.NewMemMapFs()
	cd, err := config.GetUpConfigDir()
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve up config")
	}

	tmpls := ParseTemplates(claudeTemplate, claudeRoot)

	tmplData := templateData{
		ProjectName: c.proj.Name,
		UpConfigDir: cd,
	}

	if err := renderTemplates(targetFS, tmpls, tmplData); err != nil {
		return nil, err
	}

	return targetFS, nil
}

func (c *rulesCmd) generateCodexTemplates() (afero.Fs, error) {
	targetFS := afero.NewMemMapFs()
	cd, err := config.GetUpConfigDir()
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve up config")
	}

	tmpls := ParseTemplates(codexTemplate, codexRoot)

	tmplData := templateData{
		ProjectName: c.proj.Name,
		UpConfigDir: cd,
	}

	if err := renderTemplates(targetFS, tmpls, tmplData); err != nil {
		return nil, err
	}

	return targetFS, nil
}

func ParseTemplates(f embed.FS, dir string) map[string]*template.Template {
	tpls := map[string]*template.Template{}
	err := fs.WalkDir(f, dir, func(path string, d fs.DirEntry, err error) error {
		if !d.IsDir() {
			// Use Rel to remove the "templates" directory from the path.
			s, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			// Init a new template if we don't have one for this path yet.
			t, ok := tpls[s]
			if !ok {
				t = template.New(filepath.Base(path))
			}
			// Parse files into the existing template.
			_, err = t.ParseFS(f, path)
			if err != nil {
				return err
			}
			tpls[s] = t
		}

		return err
	})

	if err != nil {
		panic(err)
	}

	return tpls
}
