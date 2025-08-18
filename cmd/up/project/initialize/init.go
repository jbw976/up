// Copyright 2025 Upbound Inc.
// All rights reserved

// Package initialize provides commands for initializing new development projects.
package initialize

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	gotemplate "text/template"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/pterm/pterm"
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/cmd/up/project/initialize/wizard"
	"github.com/upbound/up/cmd/up/project/initialize/wizard/template"
	"github.com/upbound/up/cmd/up/runner"
	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/internal/git"
	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/pkg/apis/project/v2alpha1"

	_ "embed"
)

const projectNotesPath = "NOTES.txt"

// Cmd represents the command for initializing a new project. It handles the creation
// of new projects from templates or scratch.
type Cmd struct {
	Name      string `arg:""                                                                                        help:"The name of the new project to initialize."`
	Directory string `help:"The directory to initialize. It must be empty. It will be created if it doesn't exist." type:"path"`

	Scratch      bool              `aliases:"empty"                                  default:"false"                                           help:"Create a new project from scratch." telemetry:"true"`
	Template     string            `default:""                                       help:"The template to use to initialize the new project." short:"t"`
	Values       map[string]string `help:"Values to use for templating the project."`
	StateFile    string            `default:".up_wizard_state.json"                  help:"Path to wizard state file."`
	Language     string            `default:""                                       help:"The language to use to initialize the new project." short:"l"                                 telemetry:"true"`
	TestLanguage string            `default:""                                       help:"The language to use for tests in the new project."  telemetry:"true"`

	SSHKey   string `help:"Optional. Specify an SSH key for authentication when initializing the new package. Used when transport protocol is 'ssh'."`
	Username string `help:"Optional. Specify a username for authentication. Used when transport protocol is 'https' and an SSH key is not provided, or with an SSH key when the transport protocol is 'ssh'."`
	Password string `help:"Optional. Specify a password for authentication. Used with the username when the transport protocol is 'https', or with an SSH key that requires a passphrase when the transport protocol is 'ssh'."`

	protocol        string
	gitAuthProvider git.AuthProvider
	gitCloner       git.Cloner
	projFS          afero.Fs
	projFile        string

	projDirPath string
	paths       *v2alpha1.ProjectPaths
	statePath   string
	runner      runner.CommandRunner
}

//go:embed help/init.md.tmpl
var initHelpTemplate string

// Help returns the help text for the initialize command, including examples and
// supported template options.
func (c *Cmd) Help() string {
	data := struct {
		Languages     map[string]wizard.FunctionLanguage
		TestLanguages map[string]wizard.FunctionLanguage
	}{
		Languages:     wizard.SupportedLanguagesMap,
		TestLanguages: wizard.SupportedTestLanguagesMap,
	}

	tmpl := gotemplate.Must(gotemplate.New("help").Parse(initHelpTemplate))

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return err.Error()
	}
	return buf.String()
}

// AfterApply performs validation and setup after the command flags have been parsed.
// It configures authentication, validates inputs, and sets up the project filesystem.
func (c *Cmd) AfterApply(cmdRunner runner.CommandRunner) error {
	if err := c.detectProtocol(); err != nil {
		return err
	}

	// set up the auth method based on the protocol
	switch c.protocol {
	case "ssh":
		if len(c.SSHKey) == 0 {
			return errors.New("SSH key must be specified when using SSH protocol")
		}
		c.gitAuthProvider = &git.SSHAuthProvider{
			Username:       c.Username,
			PrivateKeyPath: c.SSHKey,
			Passphrase:     c.Password, // Assuming password is used as passphrase
		}

	case "https":
		if len(c.SSHKey) > 0 {
			return errors.New("cannot specify SSH key when using HTTPS protocol")
		}
		c.gitAuthProvider = &git.HTTPSAuthProvider{
			Username: c.Username,
			Password: c.Password,
		}
	default:
		c.gitAuthProvider = &git.HTTPSAuthProvider{}
	}

	if c.Scratch {
		if c.Template != "" {
			return errors.New("cannot specify both scratch and template")
		}

		c.Template = string(wizard.BlankProjectTemplate)
		c.Language = string(wizard.FunctionLanguageKCL)
		c.TestLanguage = string(wizard.FunctionLanguageKCL)
	} else if c.Template != "" && c.Language == "" {
		return errors.New("language must be specified when using a template")
	}

	// Validate and set test language
	if c.Language != "" && c.TestLanguage == "" {
		// If no test language specified, use the main language if it's supported for tests
		if !slices.Contains(wizard.SupportedTestLanguages, c.Language) {
			return errors.New("the --language you specified is not supported for tests. Please supply a supported language for tests using the --test-language flag. Supported languages for tests are: " + strings.Join(wizard.SupportedTestLanguages, ", "))
		}
		c.TestLanguage = c.Language
	} else if c.TestLanguage != "" && !slices.Contains(wizard.SupportedTestLanguages, c.TestLanguage) {
		// Validate explicitly provided test language
		return errors.New("the --test-language you specified is not supported. Supported languages for tests are: " + strings.Join(wizard.SupportedTestLanguages, ", "))
	}

	c.gitCloner = &git.DefaultCloner{}

	// The project name must be a valid k8s resource name, which also makes it a
	// valid OCI repository name.
	if errs := validation.IsDNS1035Label(c.Name); len(errs) > 0 {
		return errors.Errorf("'%s' is not a valid project name. DNS-1035 constraints: %s", c.Name, strings.Join(errs, "; "))
	}

	if c.Directory == "" {
		c.Directory = c.Name
	}

	c.projFile = "upbound.yaml"
	projFilePath, err := filepath.Abs(filepath.Join(c.Directory, c.projFile))
	if err != nil {
		return err
	}

	c.projDirPath = filepath.Dir(projFilePath)
	c.projFS = afero.NewBasePathFs(afero.NewOsFs(), c.projDirPath)

	defaults := &v2alpha1.Project{}
	defaults.Default()
	c.paths = defaults.Spec.Paths

	c.statePath = filepath.Join(c.projDirPath, ".up", c.StateFile)
	c.runner = cmdRunner

	return nil
}

// we support local file, https, and ssh transport protocols. Infer the protocol from the template url.
func (c *Cmd) detectProtocol() error {
	repoURL := c.Template

	if repoURL == "" {
		return nil
	}

	// if the repoUrl has an explicit protocol, use that.
	if protocolSeparator := strings.Index(repoURL, "://"); protocolSeparator > -1 {
		protocol := repoURL[:protocolSeparator]

		// only support file, https, and ssh.
		if protocol == "file" || protocol == "https" || protocol == "ssh" {
			c.protocol = protocol
			return nil
		}

		// unsupported protocol
		return errors.Errorf("unsupported protocol %s in template url", protocol)
	}

	// ssh urls can be structured as [<user>@]<host>:/<path-to-git-repo>, recognized as no slashes before the first colon.
	if template.IsSSHShortURL(repoURL) {
		c.protocol = "ssh"
		return nil
	}

	// file urls are recognized as /path/to/repo.git/ or ./path/to/repo.git/ for relative paths.
	if strings.HasPrefix(repoURL, "/") || strings.HasPrefix(repoURL, ".") {
		c.protocol = "file"
		return nil
	}

	// everything else is assumed to be https.
	c.protocol = "https"
	return nil
}

// Run executes the project initialization process, handling template cloning,
// language selection, and project file generation.
func (c *Cmd) Run(ctx context.Context, upCtx *upbound.Context, p pterm.TextPrinter) error {
	var wiz *wizardResult

	if !c.Scratch && c.Template == "" {
		var err error
		wiz, err = c.runWizard()
		if err != nil {
			return err
		}

		c.Scratch = wiz.state.Template == string(wizard.BlankProjectTemplate)
		c.Template = wiz.state.Template
		c.Language = string(wiz.state.FuncLang)
		c.TestLanguage = string(wiz.state.TestLang)
	}

	pterm.Info.Printfln("Initializing project from template %q...", c.Template)
	ref, err := c.initializeProjectFromTemplate(upCtx, p)
	if err != nil {
		return err
	}
	repoURL := template.ResolveTemplateURL(c.Template).URL

	// if we got here from the wizard, we need to generate the resources
	if wiz != nil {
		err = wiz.wizard.GenerateResources(wiz.state)
		if err != nil {
			return err
		}
	}

	if err := c.updateProject(ctx, upCtx); err != nil {
		return err
	}

	pterm.Success.Printfln("Successfully initialized project %q in directory %q from %s (%s)",
		c.Name, filesystem.FullPath(c.projFS, ""), repoURL, ref.Name().Short())

	// if we got here from the wizard, we need to print the next steps
	if wiz != nil && c.Scratch {
		wiz.wizard.PrintNextSteps(wiz.state)
	}

	if !c.Scratch {
		notes, err := c.getProjectNotes()
		if err != nil {
			return err
		}
		if notes != "" {
			pterm.Info.Println("Notes:")
			pterm.Info.Println(notes)
		}
	}

	return nil
}

type wizardResult struct {
	wizard *wizard.Wizard
	state  wizard.State
}

func (c *Cmd) runWizard() (*wizardResult, error) {
	w := &wizard.Wizard{
		StatePath:   c.statePath,
		Runner:      c.runner,
		Paths:       c.paths,
		ProjectFile: filepath.Join(c.Directory, c.projFile),
		ProjectFS:   c.projFS,
	}

	result := &wizardResult{
		wizard: w,
	}

	var err error
	result.state, err = w.Run()

	return result, err
}

func (c *Cmd) initializeProjectFromTemplate(upCtx *upbound.Context, p pterm.TextPrinter) (*plumbing.Reference, error) {
	// Resolve template URL
	templateURL := template.ResolveTemplateURL(c.Template)

	// Check if target directory is suitable
	if err := c.checkTargetDirectory(c.Directory); err != nil {
		return nil, err
	}

	// Clone and transform the template
	p.Printfln("Initializing project from template %s for %s...", c.Template, c.Language)

	cloner := template.NewCloner(templateURL, c.Directory, c.Language, c.TestLanguage, c.Values, c.gitCloner, c.gitAuthProvider, p, upCtx.DebugLevel > 0)
	return cloner.CloneAndTransform()
}

func (c *Cmd) checkTargetDirectory(dir string) error {
	if _, err := os.Stat(dir); err == nil {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return errors.Wrap(err, "failed to read directory")
		}
		// Filter out the .up directory from the entries
		var nonUpEntries []os.DirEntry
		for _, entry := range entries {
			if entry.Name() != ".up" {
				nonUpEntries = append(nonUpEntries, entry)
			}
		}
		if len(nonUpEntries) > 0 {
			return errors.New("directory is not empty")
		}
	}
	return nil
}

func (c *Cmd) updateProject(ctx context.Context, upCtx *upbound.Context) error {
	var newRepo string
	if upCtx != nil && upCtx.Organization != "" {
		newRepo = fmt.Sprintf("%s/%s/%s", upCtx.RegistryEndpoint.Hostname(), upCtx.Organization, c.Name)
	} else {
		// Use "example" as the default organization because (a) it's
		// obvious-ish that it should be replaced, and (b) it's a reserved
		// account name in Upbound Cloud.
		newRepo = fmt.Sprintf("%s/example/%s", upCtx.RegistryEndpoint.Hostname(), c.Name)
	}

	if err := project.Update(c.projFS, c.projFile, func(proj *v2alpha1.Project) {
		proj.ObjectMeta.Name = c.Name
	}); err != nil {
		return errors.Wrap(err, "failed to update project metadata")
	}

	proj, err := project.Parse(c.projFS, c.projFile)
	if err != nil {
		return err
	}
	proj.Default()

	if err := project.Move(ctx, proj, c.projFS, newRepo); err != nil {
		return errors.Wrap(err, "failed to update project repository")
	}

	return nil
}

func (c *Cmd) getProjectNotes() (string, error) {
	notes, err := c.projFS.Open(projectNotesPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", errors.Wrap(err, "failed to open project notes")
	}
	defer notes.Close() //nolint:errcheck // we don't care about the error here since we're just reading the file

	notesContent, err := io.ReadAll(notes)
	if err != nil {
		return "", errors.Wrap(err, "failed to read project notes")
	}
	return string(notesContent), nil
}
