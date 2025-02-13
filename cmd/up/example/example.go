// Copyright 2025 Upbound Inc.
// All rights reserved

// Package example provides the `up example` commands.
package example

// Cmd contains commands for example cmd.
type Cmd struct {
	Generate generateCmd `cmd:"" help:"Generate an Example Claim (XRC) or Composite Resource (XR)."`
}
