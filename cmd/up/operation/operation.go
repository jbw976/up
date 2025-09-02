// Copyright 2025 Upbound Inc.
// All rights reserved

// Package operation contains the `up operation` commands.
package operation

import "github.com/upbound/up/internal/upbound"

// Cmd contains commands for operation cmd.
type Cmd struct {
	upbound.RequiresContext

	Generate generateCmd `cmd:"" help:"Generate an Operation."`
	Render   renderCmd   `cmd:"" help:"Render an Operation."`
}
