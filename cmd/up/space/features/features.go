// Copyright 2025 Upbound Inc.
// All rights reserved

package features

import (
	"github.com/crossplane/crossplane-runtime/pkg/feature"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
)

const (
	// EnableAlphaSharedTelemetry enables alpha support for Telemetry.
	EnableAlphaSharedTelemetry feature.Flag = "EnableSharedTelemetry"
	// EnableAlphaQueryAPI enables alpha support for Query API.
	EnableAlphaQueryAPI feature.Flag = "EnableQueryAPI"
)

func EnableFeatures(features *feature.Flags, params map[string]any) {
	if isAlphaSharedTelemetryEnabled(params) {
		features.Enable(EnableAlphaSharedTelemetry)
	}

	// We currently only enable the Query API feature if both Query API is enabled and
	// we own the postgres instance.
	if isAlphaQueryAPIEnabled(params) && isCNPGNeeded(params) {
		features.Enable(EnableAlphaQueryAPI)
	}
}

func isAlphaSharedTelemetryEnabled(params map[string]any) bool {
	enabled, err := fieldpath.Pave(params).GetBool("features.alpha.observability.enabled")
	if err != nil {
		return false
	}
	return enabled
}

func isAlphaQueryAPIEnabled(params map[string]any) bool {
	enabled, err := fieldpath.Pave(params).GetBool("features.alpha.apollo.enabled")
	if err != nil {
		return false
	}
	return enabled
}

func isCNPGNeeded(params map[string]any) bool {
	enabled, err := fieldpath.Pave(params).GetBool("features.alpha.apollo.storage.postgres.create")
	if err != nil {
		return false
	}
	return enabled
}
