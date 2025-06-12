// Copyright 2025 Upbound Inc.
// All rights reserved

// Package runner provides functionality for executing commands in a controlled
// environment with proper error handling and output management. This is used
// for up subcommands to run other up commands - for example, the `up project
// init` command wizard.
package runner

// CommandRunner can be bound to within the CLI to allow child commands to run
// other commands.
type CommandRunner interface {
	RunCommand(args []string) error
}
