// Copyright 2025 Upbound Inc.
// All rights reserved

// Package operation contains the `up operation` commands.
package operation

// Cmd contains commands for operation cmd.
type Cmd struct {
	Generate generateCmd `cmd:"" help:"Generate an Operation."`
}
