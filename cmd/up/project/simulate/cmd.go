// Copyright 2025 Upbound Inc.
// All rights reserved

// Package simulate contains commands for working with project simulations.
package simulate

// Cmd contains commands for managing project simulations.
type Cmd struct {
	Create   CreateCmd   `cmd:"" help:"Start a new project simulation and wait for the results."`
	Complete completeCmd `cmd:"" help:"Force complete an in-progress project simulation"`
	// Delete deleteCmd `cmd:"" help:"Delete a control plane simulation."`
}

// Help prints help.
func (c *Cmd) Help() string {
	return `
Manage project simulations. Simulations allow you to "simulate" what happens on
the control plane and see what would changes after applying the latest version
of an Upbound project.
`
}
