// Copyright 2025 Upbound Inc.
// All rights reserved

package manager

const lockFileName = ".lock.json"

// lock tracks the versions of sources whose schemas are present in the
// manager. It is persisted to the manager's filesystem.
type lock struct {
	Packages map[string]string `json:"packages"`
}

func newLock() *lock {
	return &lock{
		Packages: make(map[string]string),
	}
}
