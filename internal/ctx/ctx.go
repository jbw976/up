// Copyright 2025 Upbound Inc.
// All rights reserved

// Package ctx contains ctx navigation functions
package ctx

import (
	"context"

	"k8s.io/client-go/tools/clientcmd"

	ctxcmd "github.com/upbound/up/cmd/up/ctx"
	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/upbound"
)

// GetCurrentSpaceNavigation derives the state of the current navigation using
// the same process as up ctx.
func GetCurrentSpaceNavigation(ctx context.Context, upCtx *upbound.Context) (ctxcmd.NavigationState, error) {
	po := clientcmd.NewDefaultPathOptions()
	var err error

	conf, err := po.GetStartingConfig()
	if err != nil {
		return nil, err
	}
	return ctxcmd.DeriveState(ctx, upCtx, conf, kube.GetIngressHost)
}
