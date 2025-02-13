// Copyright 2025 Upbound Inc.
// All rights reserved

package controlplane

import (
	"context"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

// listCmd list control planes in an account on Upbound.
type listCmd struct {
	AllGroups bool   `default:"false" help:"List control planes across all groups."                                                                                      short:"A"`
	Group     string `default:""      help:"The control plane group that the control plane is contained in. This defaults to the group specified in the current context" short:"g"`
}

// AfterApply sets default values in command after assignment and validation.
func (c *listCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))
	// `-A` prevails over `-g`
	if c.AllGroups {
		c.Group = ""
	} else if c.Group == "" {
		ns, err := upCtx.GetCurrentContextNamespace()
		if err != nil {
			return err
		}
		c.Group = ns
	}
	return nil
}

// Run executes the list command.
func (c *listCmd) Run(ctx context.Context, printer upterm.ObjectPrinter, p pterm.TextPrinter, cl client.Client) error {
	var l spacesv1beta1.ControlPlaneList
	if err := cl.List(ctx, &l, client.InNamespace(c.Group)); err != nil {
		return errors.Wrap(err, "error getting control planes")
	}

	if len(l.Items) == 0 {
		p.Println("No control planes found")
		return nil
	}

	return tabularPrint(l.Items, printer)
}
