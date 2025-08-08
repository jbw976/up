// Copyright 2025 Upbound Inc.
// All rights reserved

// Package simulate contains commands for working with project simulations.
package simulate

import "github.com/upbound/up/internal/style"

// Cmd contains commands for managing project simulations.
type Cmd struct {
	Create   CreateCmd   `cmd:"" help:"Start a new project simulation and wait for the results."`
	Complete completeCmd `cmd:"" help:"Force complete an in-progress project simulation"`
	Delete   deleteCmd   `cmd:"" help:"Delete a control plane simulation."`
}

// Help prints help.
func (c *Cmd) Help() string {
	return style.RenderHelp(`
The <simulate> command manages project simulations. Simulations allow you to "simulate" what happens on
the control plane and see what changes would occur after applying the latest version
of an Upbound project.

## Usage Examples:

    up project simulate create <control-plane-name>
        Creates a new simulation for the specified control plane.
        Waits for the simulation to complete and shows results.

    up project simulate complete <simulation-name>
        Forces completion of an in-progress project simulation.
        Useful when a simulation is stuck or taking too long.

    up project simulate delete <simulation-name>
        Deletes the specified simulation.
        Removes simulation results and resources.
`)
}
