// Copyright 2025 Upbound Inc.
// All rights reserved

package kubeconfig

import (
	"github.com/alecthomas/kong"

	"github.com/upbound/up/internal/feature"
)

// BeforeReset is the first hook to run.
func (c *Cmd) BeforeReset(p *kong.Path, maturity feature.Maturity) error {
	return feature.HideMaturity(p, maturity)
}

// Cmd contains commands for managing control plane kubeconfig data.
type Cmd struct {
	Get getCmd `cmd:"" help:"Deprecated: Get a kubeconfig for a control plane and, if not specified otherwise, merge into kubeconfig and select context."`
}
