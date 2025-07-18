// Copyright 2025 Upbound Inc.
// All rights reserved

// Package ai contains the `up project ai` commands.
package ai

// Cmd contains commands for the ai subcommand.
type Cmd struct {
	ConfigureTools configureToolsCmd `cmd:"" help:"Generate AI tooling configurations for the project."`
}
