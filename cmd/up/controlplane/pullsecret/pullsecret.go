// Copyright 2025 Upbound Inc.
// All rights reserved

package pullsecret

import (
	"github.com/alecthomas/kong"

	"github.com/upbound/up/internal/feature"
)

// BeforeReset is the first hook to run.
func (c *Cmd) BeforeReset(p *kong.Path, maturity feature.Maturity) error {
	return feature.HideMaturity(p, maturity)
}

// Cmd contains commands for managing pull secrets.
type Cmd struct {
	Create createCmd `cmd:"" help:"Create a package pull secret."`
}
