// Copyright 2025 Upbound Inc.
// All rights reserved

// Package billing contains commands for working with billing reports.
package billing

// Cmd contains commands for managing billing operations.
type Cmd struct {
	Export exportCmd `cmd:"" help:"Export a billing report for submission to Upbound."`
}
