// Copyright 2025 Upbound Inc.
// All rights reserved

package migration

import (
	"github.com/alecthomas/kong"

	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/pkg/migration"
)

// AfterApply constructs and binds Upbound specific context to any subcommands
// that have Run() methods that receive it.
func (c *Cmd) AfterApply(kongCtx *kong.Context) error {
	cfg, err := kube.GetKubeConfig(c.Kubeconfig)
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
	Export      exportCmd      `cmd:"" help:"The 'export' command is used to export the current state of a Crossplane or Universal Crossplane (xp/uxp) control plane into an archive file. This file can then be used for migration to Upbound Managed Control Planes."`
	Import      importCmd      `cmd:"" help:"The 'import' command imports a control plane state from an archive file into an Upbound managed control plane."`
	PauseToggle pauseToggleCmd `cmd:"" help:"The 'pause-toggle' command is used to pause or unpause resources affected by a migration, ensuring that only migration-induced pauses are undone."`

	Kubeconfig string `help:"Override default kubeconfig path." type:"existingfile"`
}

// Help prints help.
func (c *Cmd) Help() string {
	return `
The 'migration' command is designed to facilitate the seamless migration of control planes from Crossplane or Universal
Crossplane (XP/UXP) environments to Upbound's Managed Control Planes.

This command simplifies the process of transferring your existing Crossplane configurations and states into the Upbound
platform, ensuring a smooth transition with minimal downtime.

For detailed information on each command and its options, use the '--help' flag with the specific command (e.g., 'up alpha migration export --help').
`
}
