// Copyright 2025 Upbound Inc.
// All rights reserved

// Package report contains commands for working with billing reports.
package report

// Cmd contains commands for managing billing report operations.
type Cmd struct {
	Update updateCmd `cmd:"" help:"Create or update an existing billing report by merging data from a local billing report tarball."`
}
