// Copyright 2025 Upbound Inc.
// All rights reserved

// Package project contains commands for working with development projects.
package project

import (
	"github.com/upbound/up/cmd/up/project/build"
	"github.com/upbound/up/cmd/up/project/move"
	"github.com/upbound/up/cmd/up/project/push"
	"github.com/upbound/up/cmd/up/project/run"
	"github.com/upbound/up/cmd/up/project/simulate"
)

// Cmd is the top-level project command.
type Cmd struct {
	Init     initCmd      `cmd:"" help:"Initialize a new project."`
	Build    build.Cmd    `cmd:"" help:"Build a project into a Crossplane package."`
	Push     push.Cmd     `cmd:"" help:"Push a project's packages to the Upbound Marketplace."`
	Run      run.Cmd      `cmd:"" help:"Run a project on a development control plane for testing."`
	Simulate simulate.Cmd `cmd:"" help:"Run a project as a simulation against an existing control plane."`
	Move     move.Cmd     `cmd:"" help:"Update the repository for a project"`
}
