// Copyright 2025 Upbound Inc.
// All rights reserved

// Package license contains the `up uxp license` command tree.
package license

import (
	"github.com/alecthomas/kong"

	"github.com/upbound/up/internal/upbound"
)

// Cmd is the `up uxp license` command.
type Cmd struct {
	Show showCmd `cmd:"" help:"Show the UXP license for a control plane."`

	Flags upbound.Flags `embed:""`
}

// AfterApply processes arguments and sets defaults.
func (c *Cmd) AfterApply(kongCtx *kong.Context) error {
	upCtx, err := upbound.NewFromFlags(c.Flags)
	if err != nil {
		return err
	}
	upCtx.SetupLogging()

	kongCtx.Bind(upCtx)

	return nil
}
