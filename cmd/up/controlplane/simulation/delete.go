// Copyright 2025 Upbound Inc.
// All rights reserved

package simulation

import (
	"context"
	"fmt"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	spacesv1alpha1 "github.com/upbound/up-sdk-go/apis/spaces/v1alpha1"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

// deleteCmd deletes a simulation on Upbound.
type deleteCmd struct {
	Name  string `arg:""     help:"Name of the simulation."                                                                                    predictor:"ctps"`
	Group string `default:"" help:"The group that the simulation is contained in. This defaults to the group specified in the current context" short:"g"`
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
func (c *deleteCmd) Run(ctx context.Context, p upterm.Printer, client client.Client) error {
	ctp := &spacesv1alpha1.Simulation{
		ObjectMeta: v1.ObjectMeta{
			Name:      c.Name,
			Namespace: c.Group,
		},
	}

	if err := client.Delete(ctx, ctp); err != nil {
		if kerrors.IsNotFound(err) {
			return fmt.Errorf("simulation %q not found", c.Name)
		}
		return errors.Wrap(err, "error deleting simulation")
	}
	p.Printfln("%s deleted", c.Name)
	return nil
}
