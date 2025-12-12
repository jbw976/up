// Copyright 2025 Upbound Inc.
// All rights reserved

package group

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	"github.com/upbound/up/internal/upterm"
)

// listCmd list groups in a space.
type listCmd struct{}

// Run executes the list command.
func (c *listCmd) Run(ctx context.Context, printer upterm.ResultPrinter, client ctrlclient.Client) error {
	// list groups
	var (
		nss        corev1.NamespaceList
		fieldNames = []string{"NAME", "PROTECTED"}
	)

	if err := client.List(ctx, &nss, ctrlclient.MatchingLabels(map[string]string{spacesv1beta1.ControlPlaneGroupLabelKey: "true"})); err != nil {
		return err
	}

	return printer.PrintObject(nss.Items, fieldNames, extractGroupFields)
}
