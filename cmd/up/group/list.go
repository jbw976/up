// Copyright 2025 Upbound Inc.
// All rights reserved

package group

import (
	"context"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	corev1 "k8s.io/api/core/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

// listCmd list groups in a space.
type listCmd struct{}

// AfterApply sets default values in command after assignment and validation.
func (c *listCmd) AfterApply(kongCtx *kong.Context) error {
	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))

	return nil
}

// Run executes the list command.
func (c *listCmd) Run(ctx context.Context, printer upterm.ObjectPrinter, upCtx *upbound.Context, client ctrlclient.Client, p pterm.TextPrinter) error { //nolint:gocyclo
	// list groups
	var nss corev1.NamespaceList
	if err := client.List(ctx, &nss, ctrlclient.MatchingLabels(map[string]string{spacesv1beta1.ControlPlaneGroupLabelKey: "true"})); err != nil {
		return err
	}

	return printer.Print(nss.Items, fieldNames, extractGroupFields)
}
