// Copyright 2025 Upbound Inc.
// All rights reserved

// Package install contains types for installs.
package install

import "helm.sh/helm/v3/pkg/chart"

// Option customizes the behavior of an install.
type Option func(*chart.Chart) error

// UpgradeOption customizes the behavior of an upgrade.
type UpgradeOption func(oldVersion string, ch *chart.Chart) error

// Manager can install and manage Upbound software in a Kubernetes cluster.
// TODO(hasheddan): support custom error types, such as AlreadyExists.
type Manager interface {
	GetCurrentVersion() (string, error)
	Install(version string, parameters map[string]any, opts ...Option) error
	Upgrade(version string, parameters map[string]any, opts ...UpgradeOption) error
	Uninstall() error
}

// ParameterParser parses install and upgrade parameters.
type ParameterParser interface {
	Parse() (map[string]any, error)
}
