// Copyright 2025 Upbound Inc.
// All rights reserved

// Package function contains the `up ai` commands.
package ai

// Cmd contains commands for ai cmd.
type Cmd struct {
	Rules rulesCmd `cmd:"" help:"Generate an AI tooling configurations for the project."`
}
