// Copyright 2025 Upbound Inc.
// All rights reserved

// Package webui contains the `up uxp web-ui` command tree.
package webui

import (
	"github.com/alecthomas/kong"

	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/upbound"
)

// Cmd contains commands for managing the UXP web UI.
type Cmd struct {
	Open openCmd `cmd:"" help:"Open the UXP web UI."`

	// Common Upbound API configuration
	Flags upbound.Flags `embed:""`
}

// AfterApply processes arguments and sets defaults.
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
