// Copyright 2024 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package project contains commands for working with development projects.
package project

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/pterm/pterm"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/git"
	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/yaml"
)

type initCmd struct {
	Name      string `arg:""                                                                                        help:"The name of the new project to initialize."`
	Template  string `default:"project-template"                                                                    help:"The template name or URL to use to initialize the new project."`
	Directory string `help:"The directory to initialize. It must be empty. It will be created if it doesn't exist." type:"path"`
	RefName   string `default:"main"                                                                                help:"The branch or tag to clone from the template repository."       name:"ref-name"`

	Method   string `default:"https"                                                                                                                                                                  enum:"ssh,https" help:"Specify the method to access the repository: 'https' or 'ssh'."`
	SSHKey   string `help:"Optional. Specify an SSH key for authentication when initializing the new package. Used when method is 'ssh'."`
	Username string `help:"Optional. Specify a username for authentication. Used when the method is 'https' and an SSH key is not provided, or with an SSH key when the method is 'ssh'."`
	Password string `help:"Optional. Specify a password for authentication. Used with the username when the method is 'https', or with an SSH key that requires a password when the method is 'ssh'."`

	Flags upbound.Flags `embed:""`

	gitAuthProvider git.AuthProvider
	gitCloner       git.Cloner
	projFS          afero.Fs
	projFile        string
}

func (c *initCmd) Help() string {
	tpl := `
This command initializes a new project using a specified template. You can use any Git repository as the template source.

You can specify the template by providing either a full Git URL or a well-known template name. The following well-known template names are supported:

%s

Examples:

  # Initialize a new project using a public template repository:
  up project init --template="project-template" example-project

  # Initialize a new project from a private template using Git token authentication:
  up project init --template="https://github.com/example/private-template.git" --method=https --username="<username>" --password="<token>" example-project

  # Initialize a new project from a private template using SSH authentication:
  up project init --template="git@github.com:upbound/project-template.git" --method=ssh --ssh-key=/Users/username/.ssh/id_rsa example-project

  # Initialize a new project from a private template using SSH authentication with an SSH key password:
  up project init --template="git@github.com:upbound/project-template.git" --method=ssh --ssh-key=/Users/username/.ssh/id_rsa --password="<ssh-key-password>" example-project
`

	b := strings.Builder{}
	for name, url := range wellKnownTemplates() {
		b.WriteString(fmt.Sprintf(" - %s (%s)\n", name, url))
	}

	return fmt.Sprintf(tpl, b.String())
}

// wellKnownTemplates are short aliases for template repositories.
func wellKnownTemplates() map[string]string {
	return map[string]string{
		"project-template":     "https://github.com/upbound/project-template",
		"project-template-ssh": "git@github.com:upbound/project-template.git",
	}
}

func (c *initCmd) AfterApply(kongCtx *kong.Context) error {
	switch c.Method {
	case "ssh":
		if len(c.SSHKey) == 0 {
			return errors.New("SSH key must be specified when using SSH method")
		}
		c.gitAuthProvider = &git.SSHAuthProvider{
			Username:       c.Username,
			PrivateKeyPath: c.SSHKey,
			Passphrase:     c.Password, // Assuming password is used as passphrase
		}

	case "https":
		if len(c.SSHKey) > 0 {
			return errors.New("cannot specify SSH key when using HTTPS method")
		}
		c.gitAuthProvider = &git.HTTPSAuthProvider{
			Username: c.Username,
			Password: c.Password,
		}
	}

	c.gitCloner = &git.DefaultCloner{}

	if c.Directory == "" {
		c.Directory = c.Name
	}

	c.projFile = "upbound.yaml"
	projFilePath, err := filepath.Abs(filepath.Join(c.Directory, c.projFile))
	if err != nil {
		return err
	}

	projDirPath := filepath.Dir(projFilePath)
	c.projFS = afero.NewBasePathFs(afero.NewOsFs(), projDirPath)

	upCtx, err := upbound.NewFromFlags(c.Flags)
	if err != nil {
		return err
	}
	upCtx.SetupLogging()
	kongCtx.Bind(upCtx)

	return nil
}

func (c *initCmd) Run(upCtx *upbound.Context, p pterm.TextPrinter) error {
	// Get repository URL
	repoURL := c.getRepositoryURL()

	// Clone the repository
	ref, err := c.gitCloner.CloneRepository(memory.NewStorage(), osfs.New(c.Directory, osfs.WithBoundOS()), c.gitAuthProvider, git.CloneOptions{
		Repo:      repoURL,
		RefName:   c.RefName,
		Directory: c.Directory,
	})
	if err != nil {
		return errors.Wrapf(err, "failed to clone repository from %q", repoURL)
	}

	if err := c.updateProjectFile(upCtx); err != nil {
		return err
	}

	if basePathFs, ok := c.projFS.(*afero.BasePathFs); ok {
		path := afero.FullBaseFsPath(basePathFs, "")
		p.Printfln("initialized project %q in directory %q from %s (%s)",
			c.Name, path, repoURL, ref.Name().Short())
	}

	return nil
}

func (c *initCmd) getRepositoryURL() string {
	repoURL, ok := wellKnownTemplates()[c.Template]
	if !ok {
		repoURL = c.Template
	}
	return repoURL
}

func (c *initCmd) updateProjectFile(upCtx *upbound.Context) error {
	proj, err := project.Parse(c.projFS, c.projFile)
	if err != nil {
		return errors.Wrap(err, "unable to parse the project from template repository")
	}

	proj.ObjectMeta.Name = c.Name
	if upCtx != nil && upCtx.Organization != "" {
		proj.Spec.Repository = fmt.Sprintf("%s/%s/%s", upCtx.RegistryEndpoint.Hostname(), upCtx.Organization, c.Name)
	} else {
		proj.Spec.Repository = fmt.Sprintf("%s/<organization>/%s", upCtx.RegistryEndpoint.Hostname(), c.Name)
	}

	modifiedProject, err := yaml.Marshal(&proj)
	if err != nil {
		return errors.Wrap(err, "could not construct project file")
	}

	err = afero.WriteFile(c.projFS, c.projFile, modifiedProject, 0o600)
	if err != nil {
		return errors.Wrap(err, "failed to write project %d")
	}

	return nil
}
