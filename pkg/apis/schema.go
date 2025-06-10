// Copyright 2025 Upbound Inc.
// All rights reserved

package apis

import (
	"context"

	"github.com/upbound/up/internal/xpkg/dep/manager"
	"github.com/upbound/up/internal/xpkg/schemarunner"
)

// GenerateSchema will generate meta apis schemas.
func GenerateSchema(_ context.Context, _ *manager.Manager, _ schemarunner.SchemaRunner) error {
	// TODO(adamwg): Reintroduce schema generation once it's been reworked.
	return nil
}
