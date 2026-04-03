// Copyright 2025 Upbound Inc.
// All rights reserved

package manager

const lockFileName = ".lock.json"

// lock tracks the versions of sources whose schemas are present in the
// manager. It is persisted to the manager's filesystem.
type lock struct {
	Packages map[string]string   `json:"packages"`
	// Files tracks which files were generated for each source, keyed by source
	// ID. This allows the manager to detect when generated files have been
	// deleted and force regeneration even when the version hasn't changed.
	Files    map[string][]string `json:"files,omitempty"`
}

func newLock() *lock {
	return &lock{
		Packages: make(map[string]string),
		Files:    make(map[string][]string),
	}
}
