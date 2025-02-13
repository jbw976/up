// Copyright 2025 Upbound Inc.
// All rights reserved

package oci

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
)

// CheckPreReleaseConstraint checks whether a given version, stripped of its pre-release suffix.
func CheckPreReleaseConstraint(constraint *semver.Constraints, version *semver.Version) bool {
	// Create a new version without the pre-release
	baseVersionStr := fmt.Sprintf("%d.%d.%d", version.Major(), version.Minor(), version.Patch())
	baseVersion, err := semver.NewVersion(baseVersionStr)
	if err != nil {
		return false
	}
	// Check the base version against the constraint
	return constraint.Check(baseVersion)
}
