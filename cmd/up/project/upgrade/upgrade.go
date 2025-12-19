// Copyright 2025 Upbound Inc.
// All rights reserved

// Package upgrade contains commands for working with project upgrades.
package upgrade

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/upterm"
	apiproject "github.com/upbound/up/pkg/apis/project"

	_ "embed"
)

// Cmd upgrades a project to a newer version.
type Cmd struct {
	ProjectFile string `default:"upbound.yaml" help:"Path to project definition file." short:"f"`
	projFS      afero.Fs
}

//go:embed help/upgrade.md
var upgradeHelp string

// Help returns help text for the upgrade command.
func (c *Cmd) Help() string {
	return upgradeHelp
}

// AfterApply constructs and binds Upbound-specific context.
func (c *Cmd) AfterApply() error {
	// Read the project file.
	projFilePath, err := filepath.Abs(c.ProjectFile)
	if err != nil {
		return err
	}
	// The location of the project file defines the root of the project.
	projDirPath := filepath.Dir(projFilePath)
	c.projFS = afero.NewBasePathFs(afero.NewOsFs(), projDirPath)

	return nil
}

// Run executes the upgrade command.
func (c *Cmd) Run(p upterm.Printer) error {
	vproj, err := apiproject.ParseVersioned(c.projFS, c.ProjectFile)
	if err != nil {
		return errors.Wrap(err, "failed to parse project")
	}

	p.Printfln("Current project version: %s", vproj.Version)

	// Check if upgrade is needed
	switch vproj.Version {
	case apiproject.VersionV1Alpha1:
		p.Println("Upgrading project from v1alpha1 to v2alpha1...")

		if err := project.UpgradeToV2(c.projFS, c.ProjectFile); err != nil {
			return fmt.Errorf("failed to upgrade project: %w", err)
		}

		p.PrintSuccess("Successfully upgraded project to v2alpha1!")
		p.PrintInfo("Project is now compatible with Crossplane v2.0.0+")

	case apiproject.VersionV2Alpha1:
		p.PrintInfo("Project is already at the latest version")
		return nil

	default:
		return fmt.Errorf("unsupported project version: %s", vproj.Version)
	}

	return nil
}
