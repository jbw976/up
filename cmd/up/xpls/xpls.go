// Copyright 2025 Upbound Inc.
// All rights reserved

package xpls

// Cmd --.
type Cmd struct {
	Serve serveCmd `cmd:"" help:"run a server for Crossplane definitions using the Language Server Protocol."`
}
