// Copyright 2025 Upbound Inc.
// All rights reserved

// Package billing contains commands for working with billing reports.
package billing

import "github.com/upbound/up/cmd/up/space/billing/report"

// Cmd contains commands for managing billing operations.
type Cmd struct {
	Export exportCmd  `cmd:"" help:"Export a billing report for submission to Upbound."`
	Report report.Cmd `cmd:"" help:"Manage billing reports."`
}
