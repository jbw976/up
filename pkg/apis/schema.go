// Copyright 2025 Upbound Inc.
// All rights reserved

package apis

import (
	"context"

	"github.com/upbound/up/internal/schemas/runner"
	"github.com/upbound/up/internal/xpkg/dep/manager"
)

// GenerateSchema will generate meta apis schemas.
func GenerateSchema(_ context.Context, _ *manager.Manager, _ runner.SchemaRunner) error {
	// TODO(adamwg): Reintroduce schema generation once it's been reworked.
	return nil
}
