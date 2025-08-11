// Copyright 2025 Upbound Inc.
// All rights reserved

package migration

import (
	"os"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"

	"github.com/upbound/up/cmd/up/controlplane/requires"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/pkg/migration"

	_ "embed"
)

// AfterApply constructs and binds Upbound specific context to any subcommands
// that have Run() methods that receive it.
func (c *Cmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	// Check if this is invoked via the alpha command
	if strings.HasPrefix(kongCtx.Command(), "alpha migration") {
		pterm.Warning.WithWriter(os.Stderr).Printf("The 'up alpha migration' command is deprecated and will be removed in a future release. Please use 'up controlplane migration' instead.\n\n")
	}

	cfg, err := upCtx.GetKubeconfig()
	if err != nil {
		return err
	}

	kongCtx.Bind(&migration.Context{
		Kubeconfig: cfg,
	})
	return nil
}

// Cmd contains commands for migration.
type Cmd struct {
	requires.ControlPlane

	Export      exportCmd      `cmd:"" help:"The 'export' command is used to export the current state of a Crossplane or Universal Crossplane (xp/uxp) control plane into an archive file. This file can then be used for migration to Upbound Managed Control Planes."`
	Import      importCmd      `cmd:"" help:"The 'import' command imports a control plane state from an archive file into an Upbound managed control plane."`
	PauseToggle pauseToggleCmd `cmd:"" help:"The 'pause-toggle' command is used to pause or unpause resources affected by a migration, ensuring that only migration-induced pauses are undone."`
}

//go:embed help/migration.md
var migrationHelp string

// Help prints help.
func (c *Cmd) Help() string {
	return migrationHelp
}
