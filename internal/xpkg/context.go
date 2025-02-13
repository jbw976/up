// Copyright 2025 Upbound Inc.
// All rights reserved

package xpkg

import metav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"

// InitContext defines the details we're interested in for populating a meta file.
type InitContext struct {
	// Name of the package
	Name string
	// Controller Image (only applicable to Provider packages)
	CtrlImage string
	// Crossplane version this package is compatible with
	XPVersion string
	// Dependencies for this package
	DependsOn []metav1.Dependency
}
