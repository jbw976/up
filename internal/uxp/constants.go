// Copyright 2025 Upbound Inc.
// All rights reserved

// Package uxp contains shared constants and utilities for UXP management.
package uxp

import "net/url"

const (
	// ChartName is the name of the UXP helm chart.
	ChartName = "crossplane"
	// ChartNamespace is the namespace where UXP is installed.
	ChartNamespace = "crossplane-system"
	// ImagePullSecret is the name of the image pull secret.
	ImagePullSecret = "upbound-pull-secret"
)

var (
	// RepoURL is the URL of the stable helm chart repository.
	//
	// TODO(adamwg): Change this to the stable non-OCI repo once UXPv2 is
	// released there.
	//
	//nolint:gochecknoglobals // Would make this a const if possible.
	RepoURL, _ = url.Parse("oci://xpkg.upbound.io/upbound")
	// UnstableRepoURL is the URL of the unstable helm chart repository.
	//nolint:gochecknoglobals // Would make this a const if possible.
	UnstableRepoURL, _ = url.Parse("https://charts.upbound.io/main")
)

// BaseValues returns base values for the UXP chart.
func BaseValues() map[string]any {
	return map[string]any{}
}
