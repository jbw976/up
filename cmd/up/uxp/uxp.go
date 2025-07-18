// Copyright 2025 Upbound Inc.
// All rights reserved

// Package uxp contains commands for working with UXP.
package uxp

import (
	"github.com/alecthomas/kong"

	"github.com/upbound/up/cmd/up/uxp/license"
	"github.com/upbound/up/cmd/up/uxp/webui"
	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/upbound"
)

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *Cmd) AfterApply(kongCtx *kong.Context) error {
	upCtx, err := upbound.NewFromFlags(c.Flags)
	if err != nil {
		return err
	}
	upCtx.SetupLogging()

	kubeconfig, err := upCtx.GetKubeconfig()
	if err != nil {
		return err
	}
	kongCtx.Bind(&install.Context{Kubeconfig: kubeconfig})
	return nil
}

// Cmd contains commands for managing UXP.
type Cmd struct {
	Install   installCmd   `cmd:"" help:"Install UXP."`
	Uninstall uninstallCmd `cmd:"" help:"Uninstall UXP."`
	Upgrade   upgradeCmd   `cmd:"" help:"Upgrade UXP."`
	License   license.Cmd  `cmd:"" help:"Manage UXP licenses."`

	WebUI webui.Cmd `cmd:"" help:"Manage the UXP web UI."`

	// Common Upbound API configuration
	Flags upbound.Flags `embed:""`
}
