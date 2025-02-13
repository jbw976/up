// Copyright 2025 Upbound Inc.
// All rights reserved

// Package test contains commands for working with tests project.
package test

// Cmd is the top-level project command.
type Cmd struct {
	Run      runCmd      `cmd:"" help:"Run project tests."`
	Generate generateCmd `cmd:"" help:"Generate a Test for a project."`
}
