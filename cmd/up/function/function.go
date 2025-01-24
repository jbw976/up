// Copyright 2025 Upbound Inc.
// All rights reserved

// Package function contains the `up function` commands.
package function

// Cmd contains commands for function cmd.
type Cmd struct {
	Generate generateCmd `cmd:"" help:"Generate an Function for a Composition."`
}
