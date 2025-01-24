// Copyright 2025 Upbound Inc.
// All rights reserved

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
	proj.Default()

	if err := project.Move(ctx, proj, c.projFS, c.newRepo); err != nil {
		return err
	}

	return nil
}
