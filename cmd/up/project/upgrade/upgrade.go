// Copyright 2025 Upbound Inc.
// All rights reserved

// Package upgrade contains commands for working with project upgrades.
package upgrade

import (
	"fmt"
	"path/filepath"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	"github.com/spf13/afero"

	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/upbound"
	apiproject "github.com/upbound/up/pkg/apis/project"
)

// Cmd upgrades a project to a newer version.
type Cmd struct {
	ProjectFile string `default:"upbound.yaml" help:"Path to project definition file." short:"f"`
	projFS      afero.Fs

	Flags upbound.Flags `embed:""`
}

// Help returns help text for the upgrade command.
func (c *Cmd) Help() string {
	return `
The 'upgrade' command upgrades a project to a newer API version.

Usage Examples:
    project upgrade
        Upgrades the project in the current directory to the latest supported version.

    project upgrade --project-file custom-project.yaml
        Upgrades a project with a custom file name.

Currently supported upgrades:
- v1alpha1 → v2alpha1: Adds Crossplane v2 features
`
}

// AfterApply constructs and binds Upbound-specific context.
func (c *Cmd) AfterApply(kongCtx *kong.Context) error {
	kongCtx.Bind(pterm.DefaultBulletList.WithWriter(kongCtx.Stdout))
	upCtx, err := upbound.NewFromFlags(c.Flags)
	if err != nil {
		return err
	}
	upCtx.SetupLogging()
	kongCtx.Bind(upCtx)

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
func (c *Cmd) Run(p pterm.TextPrinter) error {
	currentVersion, err := apiproject.DetectVersion(c.projFS, c.ProjectFile)
	if err != nil {
		return fmt.Errorf("failed to detect project version: %w", err)
	}

	p.Printfln("Current project version: %s", currentVersion)

	// Check if upgrade is needed
	switch currentVersion {
	case apiproject.VersionV1Alpha1:
		pterm.Println("Upgrading project from v1alpha1 to v2alpha1...")

		if err := project.UpgradeToV2(c.projFS, c.ProjectFile); err != nil {
			return fmt.Errorf("failed to upgrade project: %w", err)
		}

		pterm.Success.Println("Successfully upgraded project to v2alpha1!")
		pterm.Info.Println("Project is now compatible with Crossplane v2.0.0+")

	case apiproject.VersionV2Alpha1:
		pterm.Info.Printfln("Project is already at the latest version")
		return nil

	default:
		return fmt.Errorf("unsupported project version: %s", currentVersion)
	}

	return nil
}
