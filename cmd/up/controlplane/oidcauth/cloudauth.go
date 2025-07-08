// Copyright 2025 Upbound Inc.
// All rights reserved

// Package oidcauth contains commands for creating cloud iam roles and trust.
package oidcauth

import "github.com/upbound/up/cmd/up/controlplane/requires"

// Cmd contains commands for setup ProviderConfig and Cloud Resources with OIDC identity trust.
type Cmd struct {
	requires.SpacesControlPlane

	AWS awsCmd `cmd:"" help:"Create OIDC ProviderConfig and AWS Resources"`
}
