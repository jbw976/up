// Copyright 2025 Upbound Inc.
// All rights reserved

// Package uxp contains shared constants and utilities for UXP management.
package uxp

import (
	"net/url"
	"regexp"
)

const (
	// ChartName is the name of the UXP helm chart.
	ChartName = "crossplane"
	// ChartNamespace is the namespace where UXP is installed.
	ChartNamespace = "crossplane-system"
	// ImagePullSecret is the name of the image pull secret.
	ImagePullSecret = "upbound-pull-secret"
)

// RepoURL is the URL of the UXP helm chart repository.
//
//nolint:gochecknoglobals // Would make this a const if possible.
var RepoURL, _ = url.Parse("oci://xpkg.upbound.io/upbound")

// BaseValues returns base values for the UXP chart.
func BaseValues() map[string]any {
	return map[string]any{}
}

// StableVersionFilter returns true if v is a stable UXP version.
func StableVersionFilter(v string) bool {
	re := regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+-up\.[0-9]+$`)
	return re.MatchString(v)
}

// UnstableVersionFilter returns true unconditionally, allowing for unstable
// versions to pass through.
func UnstableVersionFilter(_ string) bool {
	return true
}
