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

// Package move provides the `up project move` command.
package move

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/alecthomas/kong"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/upbound"
)

// Cmd is the `up project move` command.
type Cmd struct {
	NewRepository string        `arg:""                 help:"The new repository for the project."`
	ProjectFile   string        `default:"upbound.yaml" help:"Path to the project definition file." short:"f"`
	Flags         upbound.Flags `embed:""`

	newRepo string
	projFS  afero.Fs
}

// AfterApply processes flags and sets defaults.
func (c *Cmd) AfterApply(kongCtx *kong.Context) error {
	upCtx, err := upbound.NewFromFlags(c.Flags)
	if err != nil {
		return err
	}
	upCtx.SetupLogging()
	kongCtx.Bind(upCtx)

	// Make sure the new repository name is valid, and apply the default
	// registry if the user didn't provide a full path.
	ref, org, repoName, err := upbound.ParseRepository(c.NewRepository, upCtx.RegistryEndpoint.Host)
	if err != nil {
		return errors.Wrap(err, "failed to parse new repository")
	}
	c.newRepo = fmt.Sprintf("%s/%s/%s", ref, org, repoName)

	// The location of the project file defines the root of the project.
	projFilePath, err := filepath.Abs(c.ProjectFile)
	if err != nil {
		return err
	}
	projDirPath := filepath.Dir(projFilePath)
	c.projFS = afero.NewBasePathFs(afero.NewOsFs(), projDirPath)

	return nil
}

// Run is the body of the command.
func (c *Cmd) Run(ctx context.Context) error {
	projFilePath := filepath.Join("/", filepath.Base(c.ProjectFile))
	proj, err := project.Parse(c.projFS, projFilePath)
	if err != nil {
		return errors.Wrap(err, "failed to parse project file")
	}

	if err := project.Move(ctx, proj, c.projFS, c.newRepo); err != nil {
		return err
	}

	return nil
}
