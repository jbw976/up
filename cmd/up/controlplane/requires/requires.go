// Copyright 2025 Upbound Inc.
// All rights reserved

// Package requires implements context requirement checks for controlplane
// subcommands. These live in a separate package to avoid circular imports
// between the controlplane package and its subcommand packages.
package requires

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	intctx "github.com/upbound/up/internal/ctx"
	"github.com/upbound/up/internal/upbound"
)

// Checker checks context requirements and configures a kube client
// appropriately.
type Checker interface {
	Check(ctx context.Context, upCtx *upbound.Context) (client.Client, error)
}

// ControlPlane requires that the current context be a control plane. It may be
// a Spaces control plane or any non-Spaces context.
type ControlPlane struct{}

// Check checks the requirement.
func (c ControlPlane) Check(ctx context.Context, upCtx *upbound.Context) (client.Client, error) {
	// Check whether we're in a space. We don't have to be, but if we are we
	// verify that our context is a control plane.
	_, _, err := intctx.GetCurrentGroup(ctx, upCtx)
	isSpace := err == nil
	// Get the current spaces context, which we'll validate in cases where we
	// are in a space and ignore otherwise.
	_, ctp, _ := upCtx.GetCurrentSpaceContextScope()

	if isSpace && ctp.Name == "" {
		return nil, errors.New("current kubeconfig context is not a control plane. Use 'up ctx' to set a control plane context")
	}

	return upCtx.BuildCurrentContextClient()
}

// SpacesControlPlane requires that the current context be a Spaces control
// plane. Arbitrary control planes will not satisfy it.
type SpacesControlPlane struct{}

// Check checks the requirement.
func (c SpacesControlPlane) Check(ctx context.Context, upCtx *upbound.Context) (client.Client, error) {
	// Check whether we're in a space. We don't have to be, but if we are we
	// verify that our context is a control plane.
	_, _, err := intctx.GetCurrentGroup(ctx, upCtx)
	isSpace := err == nil
	// Get the current spaces context, which we'll validate in cases where we
	// are in a space and ignore otherwise.
	_, ctp, _ := upCtx.GetCurrentSpaceContextScope()

	if !isSpace || ctp.Name == "" {
		return nil, errors.New("current kubeconfig context is not a Spaces control plane. Use 'up ctx' to set a control plane context")
	}

	return upCtx.BuildCurrentContextClient()
}

// Space requires that the current context is a space (or group within a
// space). The parent group will be used if the current context is a control
// plane.
type Space struct{}

// Check checks the requirement.
func (c Space) Check(ctx context.Context, upCtx *upbound.Context) (client.Client, error) {
	// Check whether we're in a space. We don't have to be, but if we are we
	// verify that our context is a control plane.
	_, _, err := intctx.GetCurrentGroup(ctx, upCtx)
	isSpace := err == nil

	if !isSpace {
		return nil, errors.New("current kubeconfig context is not in an Upbound Space. Use 'up ctx' to set an Upbound context")
	}

	kubeconfig, err := intctx.GetSpacesKubeconfig(ctx, upCtx)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get kubeconfig for current spaces context")
	}
	restConfig, err := kubeconfig.ClientConfig()
	if err != nil {
		return nil, errors.Wrap(err, "cannot get rest config for spaces client")
	}

	return client.New(restConfig, client.Options{})
}
