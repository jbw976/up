// Copyright 2025 Upbound Inc.
// All rights reserved

// Package xrd provides the `up xrd` commands.
package xrd

// Cmd contains commands for xrd cmd.
type Cmd struct {
	Generate generateCmd `cmd:"" help:"Generate an XRD from a Composite Resource (XR) or Claim (XRC)."`
}
