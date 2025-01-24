// Copyright 2025 Upbound Inc.
// All rights reserved

// Package connector contains commands for working with the mcp-connector.
package connector

import (
	"github.com/alecthomas/kong"

	"github.com/upbound/up/internal/feature"
)

// BeforeReset is the first hook to run.
func (c *Cmd) BeforeReset(p *kong.Path, maturity feature.Maturity) error {
	return feature.HideMaturity(p, maturity)
}

// Cmd contains commands for installing mcp-connector into an App Cluster.
type Cmd struct {
	Install   installCmd   `cmd:"" help:"Install mcp-connector into an App Cluster."`
	Uninstall uninstallCmd `cmd:"" help:"Uninstall mcp-connector from an App Cluster."`
}
