// Copyright 2025 Upbound Inc.
// All rights reserved

// Package resource contains commands for managing Crossplane resources.
package resource

// Cmd is the root command for resource operations.
type Cmd struct {
	Count countCmd `cmd:"" help:"Count Crossplane resources in a cluster."`
}
