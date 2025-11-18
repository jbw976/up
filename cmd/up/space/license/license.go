// Copyright 2025 Upbound Inc.
// All rights reserved

// Package license contains the `up space license` command tree.
package license

import (
	"github.com/alecthomas/kong"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	adminv1alpha1 "github.com/upbound/up-sdk-go/apis/admin/v1alpha1"
	"github.com/upbound/up/internal/upbound"
)

// Cmd is the `up space license` command.
type Cmd struct {
	upbound.RequiresContext `embed:""`

	Show   showCmd   `cmd:"" help:"Show the Space license."`
	Apply  applyCmd  `cmd:"" help:"Apply a Space license. Specify either a license file or use --dev for development clusters."`
	Remove removeCmd `cmd:"" help:"Remove the Space license."`
}

// AfterApply processes arguments and sets defaults.
func (c *Cmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	cl, err := upCtx.BuildCurrentContextClient()
	if err != nil {
		return errors.Wrap(err, "failed to get kube client")
	}

	if err := adminv1alpha1.AddToScheme(cl.Scheme()); err != nil {
		return errors.Wrap(err, "failed to add license types to scheme")
	}

	kongCtx.BindTo(cl, (*client.Client)(nil))
	kongCtx.BindTo(upCtx, (*upbound.Context)(nil))

	return nil
}
