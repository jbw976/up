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

package move

import (
	"context"
	"path/filepath"

	"github.com/alecthomas/kong"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/pterm/pterm"
	"github.com/spf13/afero"

	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/upbound"
)

type Cmd struct {
	NewRepository string        `arg:"" help:"The new repository for the project."`
	ProjectFile   string        `short:"f" help:"Path to the project definition file." default:"upbound.yaml"`
	Flags         upbound.Flags `embed:""`

	newRepo name.Repository
	projFS  afero.Fs
}

func (c *Cmd) AfterApply(kongCtx *kong.Context) error {
	upCtx, err := upbound.NewFromFlags(c.Flags)
	if err != nil {
		return err
	}
	upCtx.SetupLogging()
	kongCtx.Bind(upCtx)

	// Make sure the new repository name is valid, and apply the default
	// registry if the user didn't provide a full path.
	newRepo, err := name.NewRepository(c.NewRepository, name.WithDefaultRegistry(upCtx.RegistryEndpoint.Host))
	if err != nil {
		return errors.Wrap(err, "failed to parse new repository")
	}
	c.newRepo = newRepo

	// The location of the project file defines the root of the project.
	projFilePath, err := filepath.Abs(c.ProjectFile)
	if err != nil {
		return err
	}
	projDirPath := filepath.Dir(projFilePath)
	c.projFS = afero.NewBasePathFs(afero.NewOsFs(), projDirPath)

	return nil
}

func (c *Cmd) Run(ctx context.Context, p pterm.TextPrinter) error {
	projFilePath := filepath.Join("/", filepath.Base(c.ProjectFile))
	proj, err := project.Parse(c.projFS, projFilePath)
	if err != nil {
		return errors.Wrap(err, "failed to parse project file")
	}

	if err := project.Move(ctx, proj, c.projFS, c.newRepo.String()); err != nil {
		return err
	}

	return nil
}
