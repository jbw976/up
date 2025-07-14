// Copyright 2025 Upbound Inc.
// All rights reserved

// Package license contains the `up uxp license` command tree.
package license

import (
	"github.com/alecthomas/kong"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/controller-manager/apis/licensing/v1alpha1"
	"github.com/upbound/up/internal/upbound"
)

// Cmd is the `up uxp license` command.
type Cmd struct {
	Show   showCmd   `cmd:"" help:"Show the UXP license for a control plane."`
	Apply  applyCmd  `cmd:"" help:"Apply a UXP license to a control plane."`
	Remove removeCmd `cmd:"" help:"Remove the UXP license from a control plane."`

	Flags upbound.Flags `embed:""`
}

// AfterApply processes arguments and sets defaults.
func (c *Cmd) AfterApply(kongCtx *kong.Context) error {
	upCtx, err := upbound.NewFromFlags(c.Flags)
	if err != nil {
		return err
	}
	upCtx.SetupLogging()

	cl, err := upCtx.BuildCurrentContextClient()
	if err != nil {
		return errors.Wrap(err, "failed to get kube client")
	}

	if err := v1alpha1.AddToScheme(cl.Scheme()); err != nil {
		return errors.Wrap(err, "failed to add license types to scheme")
	}

	kongCtx.BindTo(cl, (*client.Client)(nil))

	return nil
}
