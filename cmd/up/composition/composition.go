// Copyright 2025 Upbound Inc.
// All rights reserved

package composition

import "github.com/upbound/up/internal/upbound"

// Cmd contains commands for composition cmd.
type Cmd struct {
	upbound.RequiresContext

	Generate generateCmd `cmd:"" help:"Generate an Composition."`
	Render   renderCmd   `cmd:"" help:"Run a composition locally to render an XR into composed resources."`
}
