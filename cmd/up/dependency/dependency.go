// Copyright 2025 Upbound Inc.
// All rights reserved

package dependency

// Cmd contains commands for dependency cmd.
type Cmd struct {
	Add         addCmd         `cmd:"" help:"Add a dependency to the current project."`
	UpdateCache updateCacheCmd `cmd:"" help:"Update the dependency cache for the current project."`
	CleanCache  cleanCacheCmd  `cmd:"" help:"Clean the dependency cache."`
}

func (c *Cmd) Help() string {
	return `
The dependency command manages crossplane package dependencies of the project
in the current directory. It caches package information in a local file system
cache (by default in ~/.up/cache), to be used e.g. for the upbound language
server.
`
}
