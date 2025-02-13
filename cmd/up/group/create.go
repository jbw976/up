// Copyright 2025 Upbound Inc.
// All rights reserved

package group

import (
	"context"

	"github.com/pterm/pterm"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

// createCmd creates a group in a space.
type createCmd struct {
	Name string `arg:"" help:"Name of group." required:""`
}

// Run executes the create command.
func (c *createCmd) Run(ctx context.Context, printer upterm.ObjectPrinter, upCtx *upbound.Context, client client.Client, p pterm.TextPrinter) error { //nolint:gocyclo
	// create group
	group := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: c.Name,
			Labels: map[string]string{
				spacesv1beta1.ControlPlaneGroupLabelKey: "true",
			},
		},
	}

	if err := client.Create(ctx, &group); err != nil {
		return err
	}

	p.Printfln("%s created", c.Name)
	return nil
}
