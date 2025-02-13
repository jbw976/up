// Copyright 2025 Upbound Inc.
// All rights reserved

package group

import (
	"context"
	"fmt"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

// getCmd gets a specific group in a space.
type getCmd struct {
	Name string `arg:"" help:"Name of group." required:""`
}

// AfterApply sets default values in command after assignment and validation.
func (c *getCmd) AfterApply(kongCtx *kong.Context) error {
	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))

	return nil
}

// Run executes the list command.
func (c *getCmd) Run(ctx context.Context, printer upterm.ObjectPrinter, upCtx *upbound.Context, client client.Client, p pterm.TextPrinter) error { //nolint:gocyclo
	// list groups
	var ns corev1.Namespace
	if err := client.Get(ctx, types.NamespacedName{Name: c.Name}, &ns); err != nil {
		return err
	}

	// only print the group if it is a registered group
	if _, ok := ns.Labels[spacesv1beta1.ControlPlaneGroupLabelKey]; !ok {
		return fmt.Errorf("namespace %q is not a group", c.Name)
	}

	return printer.Print(ns, fieldNames, extractGroupFields)
}
