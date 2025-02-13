// Copyright 2025 Upbound Inc.
// All rights reserved

package controlplane

import (
	"context"
	"fmt"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	cp "github.com/upbound/up-sdk-go/service/controlplanes"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

// AfterApply sets default values in command after assignment and validation.
func (c *getCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))
	// default to group pointed by current context
	if c.Group == "" {
		ns, err := upCtx.GetCurrentContextNamespace()
		if err != nil {
			return err
		}
		c.Group = ns
	}
	return nil
}

// getCmd gets a single control plane in an account on Upbound.
type getCmd struct {
	Name  string `arg:""     help:"Name of control plane."                                                                                                      predictor:"ctps" required:""`
	Group string `default:"" help:"The control plane group that the control plane is contained in. This defaults to the group specified in the current context" short:"g"`
}

// Run executes the get command.
func (c *getCmd) Run(ctx context.Context, printer upterm.ObjectPrinter, client client.Client) error {
	var ctp spacesv1beta1.ControlPlane
	if err := client.Get(ctx, types.NamespacedName{Namespace: c.Group, Name: c.Name}, &ctp); err != nil {
		if kerrors.IsNotFound(err) {
			return fmt.Errorf("control plane %q not found", c.Name)
		}

		return errors.Wrap(err, "error getting control plane")
	}

	return tabularPrint(ctp, printer)
}

// EmptyControlPlaneConfiguration returns an empty ControlPlaneConfiguration with default values.
func EmptyControlPlaneConfiguration() cp.ControlPlaneConfiguration {
	configuration := cp.ControlPlaneConfiguration{}
	configuration.Status = cp.ConfigurationInstallationQueued
	return configuration
}
