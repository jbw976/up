// Copyright 2025 Upbound Inc.
// All rights reserved

package xpkg

import (
	"github.com/alecthomas/kong"

	"github.com/upbound/up/internal/feature"
)

// BeforeReset is the first hook to run.
func (c *Cmd) BeforeReset(p *kong.Path, maturity feature.Maturity) error {
	return feature.HideMaturity(p, maturity)
}

// Cmd contains commands for interacting with xpkgs.
type Cmd struct {
	Build     buildCmd     `cmd:"" help:"Build a package, by default from the current directory."`
	XPExtract xpExtractCmd `cmd:"" help:"Extract package contents into a Crossplane cache compatible format. Fetches from a remote registry by default." maturity:"alpha"`
	Push      pushCmd      `cmd:"" help:"Push a package."`
	Batch     batchCmd     `cmd:"" help:"Batch build and push a family of service-scoped provider packages."                                             maturity:"alpha"`
}

func (c *Cmd) Help() string {
	return `
This command is deprecated and will be removed in a future release.

To build Crossplane packages with up, use the project commands. To work with
non-project Crossplane packages, use the crossplane CLI.
`
}
