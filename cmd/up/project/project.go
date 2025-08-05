// Copyright 2025 Upbound Inc.
// All rights reserved

// Package project contains commands for working with development projects.
package project

import (
	"github.com/alecthomas/kong"

	"github.com/upbound/up/cmd/up/project/ai"
	"github.com/upbound/up/cmd/up/project/build"
	"github.com/upbound/up/cmd/up/project/initialize"
	"github.com/upbound/up/cmd/up/project/move"
	"github.com/upbound/up/cmd/up/project/push"
	"github.com/upbound/up/cmd/up/project/run"
	"github.com/upbound/up/cmd/up/project/simulate"
	"github.com/upbound/up/cmd/up/project/stop"
	"github.com/upbound/up/cmd/up/project/upgrade"
	"github.com/upbound/up/internal/upbound"
)

// Cmd is the top-level project command.
type Cmd struct {
	upbound.RequiresContext

	Init    initialize.Cmd `cmd:"" help:"Initialize a new project."`
	Build   build.Cmd      `cmd:"" help:"Build a project into a Crossplane package."`
	Push    push.Cmd       `cmd:"" help:"Push a project's packages to the Upbound Marketplace."`
	Run     run.Cmd        `cmd:"" help:"Run a project on a development control plane for testing."`
	Stop    stop.Cmd       `cmd:"" help:"Tear down a development control plane started by the run command."`
	Move    move.Cmd       `cmd:"" help:"Update the repository for a project"`
	Upgrade upgrade.Cmd    `cmd:"" help:"Upgrade a project to a newer API version."`

	Simulate   simulate.CreateCmd `cmd:"" help:"Run a project as a simulation against an existing control plane."`
	Simulation simulate.Cmd       `cmd:"" help:"Manage project simulations."`

	AI ai.Cmd `cmd:"" help:"Generate AI tooling for a project."`
}

// AfterApply sets up data for subcommands.
func (c *Cmd) AfterApply(kongCtx *kong.Context) error {
	// Give subcommands access to the raw flags. We need this in `run` to get
	// the user's kubeconfig path.
	kongCtx.Bind(c.Flags)

	return nil
}
