// Copyright 2025 Upbound Inc.
// All rights reserved

package xpkg

// Package represents the types of packages we support.
type Package string

const (
	// Configuration represents a configuration package.
	Configuration Package = "configuration"
	// Provider represents a provider package.
	Provider Package = "provider"
	// Function represents a function package.
	Function Package = "function"
)

// IsValid is a helper function for determining if the Package
// is a valid type of package.
func (p Package) IsValid() bool {
	switch p {
	case Configuration, Provider, Function:
		return true
	}
	return false
}
