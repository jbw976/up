// Copyright 2025 Upbound Inc.
// All rights reserved

// Package uxp contains commands for working with UXP.
package uxp

import (
	"net/url"

	"github.com/alecthomas/kong"

	"github.com/upbound/up/cmd/up/uxp/license"
	"github.com/upbound/up/cmd/up/uxp/webui"
	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/upbound"
)

const (
	chartName      = "crossplane"
	chartNamespace = "crossplane-system"

	imagePullSecret = "upbound-pull-secret"
)

// baseValues returns base values for the UXP chart.
func baseValues() map[string]any {
	return map[string]any{
		// TODO(branden): Remove this once UXP is public.
		"upbound": map[string]any{
			"manager": map[string]any{
				"imagePullSecrets": []map[string]any{{
					"name": imagePullSecret,
				}},
			},
		},
		"webui": map[string]any{
			"imagePullSecrets": []map[string]any{{
				"name": imagePullSecret,
			}},
		},
		"apollo": map[string]any{
			"imagePullSecrets": []map[string]any{{
				"name": imagePullSecret,
			}},
		},
	}
}

var (
	// RepoURL is the URL of the stable helm chart repository.
	//
	// TODO(adamwg): Change this to the public repo once UXPv2 is released.
	//
	//nolint:gochecknoglobals // Would make this a const if possible.
	RepoURL, _ = url.Parse("oci://xpkg.upbound.io/upbound-dev")
	//nolint:gochecknoglobals // Would make this a const if possible.
	uxpUnstableRepoURL, _ = url.Parse("https://charts.upbound.io/main")
)

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *Cmd) AfterApply(kongCtx *kong.Context) error {
	upCtx, err := upbound.NewFromFlags(c.Flags)
	if err != nil {
		return err
	}
	upCtx.SetupLogging()

	kubeconfig, err := upCtx.GetKubeconfig()
	if err != nil {
		return err
	}
	kongCtx.Bind(&install.Context{Kubeconfig: kubeconfig})
	return nil
}

// Cmd contains commands for managing UXP.
type Cmd struct {
	Install   installCmd   `cmd:"" help:"Install UXP."`
	Uninstall uninstallCmd `cmd:"" help:"Uninstall UXP."`
	Upgrade   upgradeCmd   `cmd:"" help:"Upgrade UXP."`
	License   license.Cmd  `cmd:"" help:"Manage UXP licenses."`

	WebUI webui.Cmd `cmd:"" help:"Manage the UXP web UI."`

	// Common Upbound API configuration
	Flags upbound.Flags `embed:""`
}
