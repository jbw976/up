// Copyright 2025 Upbound Inc.
// All rights reserved

package pkg

import (
	"github.com/alecthomas/kong"

	"github.com/upbound/up/internal/feature"
)

// BeforeReset is the first hook to run.
func (c *Cmd) BeforeReset(ctx *kong.Context, p *kong.Path, maturity feature.Maturity) error {
	return feature.HideMaturity(p, maturity)
}

// Cmd contains commands for managing packages in a control plane.
type Cmd struct {
	Install installCmd `cmd:"" help:"Install a ${package_type}."`
}
