// Copyright 2025 Upbound Inc.
// All rights reserved

package ai

import (
	"embed"
	"io/fs"
	"path/filepath"
	"text/template"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/pkg/apis/project/v2alpha1"
)

//go:embed help/configure.md
var configureHelp string

func (c *configureToolsCmd) Help() string {
	return configureHelp
}

var (
	//go:embed all:templates/claude
	claudeTemplate embed.FS
	//go:embed all:templates/cursor
	cursorTemplate embed.FS
	//go:embed all:templates/gemini
	geminiTemplate embed.FS
	//go:embed all:templates/copilot
	copilotTemplate embed.FS
)

const (
	// The image ref for the marketplace-mcp-server.
	imageMarketplaceMCP = "xpkg.upbound.io/upbound/marketplace-mcp-server:v0.1.0"
)

type configureToolsCmd struct {
	ProjectFile string `default:"upbound.yaml" help:"Path to project definition file." short:"f"`

	UseGemini  bool `default:"false" group:"Tooling Provider Flags:" help:"Generate gemini CLI configurations."      name:"gemini-cli"`
	UseClaude  bool `default:"false" group:"Tooling Provider Flags:" help:"Generate claude code CLI configurations." name:"claude-code"`
	UseCursor  bool `default:"false" group:"Tooling Provider Flags:" help:"Generate cursor configurations."          name:"cursor"`
	UseCopilot bool `default:"false" group:"Tooling Provider Flags:" help:"Generate GitHub Copilot configurations."  name:"copilot"`

	projFS afero.Fs
	proj   *v2alpha1.Project
}

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *configureToolsCmd) AfterApply(kongCtx *kong.Context) error {
	kongCtx.Bind(pterm.DefaultBulletList.WithWriter(kongCtx.Stdout))

	if !c.UseGemini && !c.UseClaude && !c.UseCursor && !c.UseCopilot {
		return errors.New("no tools specified; must specify at least one tool to configure")
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

	return nil
}

func (c *configureToolsCmd) Run(printer upterm.ObjectPrinter) (err error) {
	cfgFS := []afero.Fs{}
	// filepath roots for the various tool configuration sub directories.
	claudeRoot := "templates/claude"
	cursorRoot := "templates/cursor"
	geminiRoot := "templates/gemini"
	copilotRoot := "templates/copilot"

	if c.UseGemini {
		fs, err := c.generateTemplates(geminiTemplate, geminiRoot)
		if err != nil {
			return errors.Wrap(err, "failed to handle gemini templates")
		}
		cfgFS = append(cfgFS, fs)
	}

	if c.UseClaude {
		fs, err := c.generateTemplates(claudeTemplate, claudeRoot)
		if err != nil {
			return errors.Wrap(err, "failed to handle claude templates")
		}
		cfgFS = append(cfgFS, fs)
	}

	if c.UseCursor {
		fs, err := c.generateTemplates(cursorTemplate, cursorRoot)
		if err != nil {
			return errors.Wrap(err, "failed to handle cursor templates")
		}
		cfgFS = append(cfgFS, fs)
	}

	if c.UseCopilot {
		fs, err := c.generateTemplates(copilotTemplate, copilotRoot)
		if err != nil {
			return errors.Wrap(err, "failed to handle copilot templates")
		}
		cfgFS = append(cfgFS, fs)
	}

	err = upterm.WrapWithSuccessSpinner(
		"Generating AI Tool Configurations",
		func() error {
			for _, fs := range cfgFS {
				if err := filesystem.CopyFilesBetweenFs(fs, c.projFS); err != nil {
					return errors.Wrap(err, "failed to copy files to target directories")
				}
			}
			return nil
		}, printer)
	if err != nil {
		return err
	}

	pterm.Println()
	pterm.Info.WithPrefix(upterm.BotPrefix).Printfln("Successfully created tooling configurations and saved to %s", filesystem.FullPath(c.projFS, ""))
	pterm.Println()
	return nil
}

// templateData provides values for the go-templated files.
type templateData struct {
	ProjectName string
	// Path for .up/config.json
	UpConfigDir string
	// MarketPlaceMCPServer image
	MarketPlaceMCP string
}

// generateTemplates from the rootDir of the given filesystem, or errors.
func (c *configureToolsCmd) generateTemplates(fs embed.FS, rootDir string) (afero.Fs, error) {
	targetFS := afero.NewMemMapFs()
	cd, err := config.GetUpConfigDir()
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve up config")
	}

	// determine the target locations for each of the template files.
	tmpls, err := parseTemplates(fs, rootDir)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse templates for the AI configurations")
	}

	tmplData := templateData{
		ProjectName:    c.proj.Name,
		UpConfigDir:    cd,
		MarketPlaceMCP: imageMarketplaceMCP,
	}

	if err := writeTemplates(targetFS, tmpls, tmplData); err != nil {
		return nil, err
	}

	return targetFS, nil
}

// parseTemplates walks the supplied filesystem from the given dir, returning
// a map of filepath to template.Template, or errors.
func parseTemplates(f embed.FS, dir string) (map[string]*template.Template, error) {
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
		return nil, err
	}

	return tpls, nil
}

// writeTemplates to the given filepaths in the supplied tmpls map.
func writeTemplates(targetFS afero.Fs, tmpls map[string]*template.Template, data any) error {
	for path, tmpl := range tmpls {
		file, err := targetFS.Create(filepath.Clean(path))
		if err != nil {
			return errors.Wrapf(err, "error creating file %v", path)
		}

		if err := tmpl.Execute(file, data); err != nil {
			if ferr := file.Close(); ferr != nil {
				return errors.Wrap(ferr, "failed to close file")
			}
			return errors.Wrapf(err, "error writing template to file %v", path)
		}

		if ferr := file.Close(); ferr != nil {
			return errors.Wrap(ferr, "failed to close file")
		}
	}
	return nil
}
