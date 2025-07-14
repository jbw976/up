// Copyright 2025 Upbound Inc.
// All rights reserved

// Package apiconnector contains commands for working with the api-connector.
package apiconnector

import (
	"github.com/alecthomas/kong"

	"github.com/upbound/up/internal/feature"
)

// BeforeReset is the first hook to run.
func (c *Cmd) BeforeReset(p *kong.Path, maturity feature.Maturity) error {
	return feature.HideMaturity(p, maturity)
}

// Cmd contains commands for installing api-connector into an App Cluster.
type Cmd struct {
	Install   installCmd   `cmd:"" help:"Install api-connector into an App Cluster."`
	Uninstall uninstallCmd `cmd:"" help:"Uninstall api-connector from an App Cluster."`
}
