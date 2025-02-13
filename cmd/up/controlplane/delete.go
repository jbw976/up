// Copyright 2025 Upbound Inc.
// All rights reserved

package controlplane

import (
	"context"
	"fmt"

	"github.com/pterm/pterm"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	"github.com/upbound/up/internal/upbound"
)

// deleteCmd deletes a control plane on Upbound.
type deleteCmd struct {
	Name  string `arg:""     help:"Name of control plane."                                                                                                      predictor:"ctps"`
	Group string `default:"" help:"The control plane group that the control plane is contained in. This defaults to the group specified in the current context" short:"g"`
}

// AfterApply sets default values in command after assignment and validation.
func (c *deleteCmd) AfterApply(upCtx *upbound.Context) error {
	if c.Group == "" {
		ns, err := upCtx.GetCurrentContextNamespace()
		if err != nil {
			return err
		}
		c.Group = ns
	}
	return nil
}

// Run executes the delete command.
func (c *deleteCmd) Run(ctx context.Context, p pterm.TextPrinter, client client.Client) error {
	ctp := &spacesv1beta1.ControlPlane{
		ObjectMeta: v1.ObjectMeta{
			Name:      c.Name,
			Namespace: c.Group,
		},
	}

	if err := client.Delete(ctx, ctp); err != nil {
		if kerrors.IsNotFound(err) {
			return fmt.Errorf("control plane %q not found", c.Name)
		}
		return errors.Wrap(err, "error deleting control plane")
	}
	p.Printfln("%s deleted", c.Name)
	return nil
}
