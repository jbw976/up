// Copyright 2025 Upbound Inc.
// All rights reserved

package ai

import (
	"context"
	"embed"
	"fmt"
	"os/user"
	"path/filepath"
	"strings"
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

//go:embed templates/claude/**
var claudeTemplate embed.FS

//go:embed templates/codex/**
var codexTemplate embed.FS

//go:embed templates/gemini/**
var geminiTemplate embed.FS

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

func renderTemplates(targetFS afero.Fs, templates *template.Template, data any) error {
	for _, tmpl := range templates.Templates() {
		fname := tmpl.Name()
		if strings.HasPrefix(fname, "dot-") {
			fname = convertDotFile(fname)
		}
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

	templates := template.Must(template.ParseFS(geminiTemplate, "templates/gemini/**"))
	tmplData := templateData{
		ProjectName: c.proj.Name,
		UpConfigDir: cd,
	}

	if err := renderTemplates(targetFS, templates, tmplData); err != nil {
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

	templates := template.Must(template.ParseFS(claudeTemplate, "templates/claude/**"))
	tmplData := templateData{
		ProjectName: c.proj.Name,
		UpConfigDir: cd,
	}

	if err := renderTemplates(targetFS, templates, tmplData); err != nil {
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

	templates := template.Must(template.ParseFS(codexTemplate, "templates/codex/**"))
	tmplData := templateData{
		ProjectName: c.proj.Name,
		UpConfigDir: cd,
	}

	if err := renderTemplates(targetFS, templates, tmplData); err != nil {
		return nil, err
	}

	return targetFS, nil
}

// convertDotFile takes a file name of the form dot-<directory-name>-<filename>
// and returns a filepath of the form .directory-name/filename.
func convertDotFile(name string) string {
	spl := strings.Split(name, "-")
	fname := spl[len(spl)-1]
	dirname := fmt.Sprintf(".%s", spl[1])
	return filepath.Join(dirname, fname)
}
