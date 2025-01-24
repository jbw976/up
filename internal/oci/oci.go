// Copyright 2025 Upbound Inc.
// All rights reserved

// Package oci contains functions for handling remote oci artifacts
package oci

import (
	"strings"
)

// GetArtifactName extracts the artifact name from the chart reference and replaces ':' with '-'.
func GetArtifactName(artifact string) string {
	parts := strings.Split(artifact, "/")
	artifactPathName := parts[len(parts)-1]
	return strings.ReplaceAll(artifactPathName, ":", "-")
}

// RemoveDomainAndOrg removes the domain and organization from the repository URL.
func RemoveDomainAndOrg(src string) string {
	parts := strings.SplitN(src, "/", 3)
	if len(parts) == 3 {
		return parts[2]
	}
	if len(parts) == 2 {
		return parts[1]
	}
	return src
}
