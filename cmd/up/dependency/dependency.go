// Copyright 2025 Upbound Inc.
// All rights reserved

// Package dependency contains commands for managing project dependencies.
package dependency

import (
	"github.com/upbound/up/internal/upbound"

	_ "embed"
)

// Cmd contains commands for dependency cmd.
type Cmd struct {
	upbound.RequiresContext

	Add         addCmd         `cmd:"" help:"Add a dependency to the current project."`
	List        listCmd        `cmd:"" help:"List all transitive dependencies for the current project or a specific package."`
	Tree        treeCmd        `cmd:"" help:"Display the dependency tree for the current project or a specific package."`
	UpdateCache updateCacheCmd `cmd:"" help:"Update the dependency cache for the current project."`
	CleanCache  cleanCacheCmd  `cmd:"" help:"Clean the dependency cache."`
}

//go:embed help/dependency.md
var dependencyHelp string

// Help returns help.
func (c *Cmd) Help() string {
	return dependencyHelp
}
