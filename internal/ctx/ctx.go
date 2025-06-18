// Copyright 2025 Upbound Inc.
// All rights reserved

// Package ctx contains ctx navigation functions
package ctx

import (
	"context"

	"k8s.io/client-go/tools/clientcmd"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	ctxcmd "github.com/upbound/up/cmd/up/ctx"
	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/upbound"
)

// GetCurrentGroup derives the state of the current navigation using the same
// process as up ctx and returns the Space and Group. The Group will be nil if
// the current context is a Space. If the current navigation state is not in a
// space, an error is returned.
func GetCurrentGroup(ctx context.Context, upCtx *upbound.Context) (ctxcmd.Space, *ctxcmd.Group, error) {
	conf, err := upCtx.GetRawKubeconfig()
	if err != nil {
		return nil, nil, err
	}

	state, err := ctxcmd.DeriveState(ctx, upCtx, &conf, kube.GetIngressHost)
	if err != nil {
		return nil, nil, err
	}

	// Space can't go in the switch below since it's not a concrete type and we
	// don't want to know the details of different kinds of spaces here.
	if space, ok := state.(ctxcmd.Space); ok {
		return space, nil, nil
	}

	switch st := state.(type) {
	case *ctxcmd.Group:
		return st.Space, st, nil
	case *ctxcmd.ControlPlane:
		return st.Group.Space, &st.Group, nil
	}

	return nil, nil, errors.New("current kubeconfig context is not in an Upbound Space")
}

// GetSpacesKubeconfig returns a kubeconfig for the current context's Space. If
// the current context is a Spaces control plane, the default namespace will be
// the containing Group. If the current kubeconfig context is not in a Space, it
// returns an error.
func GetSpacesKubeconfig(ctx context.Context, upCtx *upbound.Context) (clientcmd.ClientConfig, error) {
	space, group, err := GetCurrentGroup(ctx, upCtx)
	if err != nil {
		return nil, err
	}

	if group == nil {
		return space.GetKubeconfig()
	}

	return group.GetKubeconfig()
}
