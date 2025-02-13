// Copyright 2025 Upbound Inc.
// All rights reserved

// Package uxp contains commands for working with UXP.
package uxp

import (
	"net/url"

	"github.com/alecthomas/kong"

	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/upbound"
)

const (
	chartName          = "universal-crossplane"
	alternateChartName = "crossplane"
)

var (
	// RepoURL is the URL of the stable helm chart repository.
	//nolint:gochecknoglobals // Would make this a const if possible.
	RepoURL, _ = url.Parse("https://charts.upbound.io/stable")
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
	kongCtx.Bind(&install.Context{
		Kubeconfig: kubeconfig,
		Namespace:  c.Namespace,
	})
	return nil
}

// Cmd contains commands for managing UXP.
type Cmd struct {
	Install   installCmd   `cmd:"" help:"Install UXP."`
	Uninstall uninstallCmd `cmd:"" help:"Uninstall UXP."`
	Upgrade   upgradeCmd   `cmd:"" help:"Upgrade UXP."`

	Namespace string `default:"upbound-system" env:"UXP_NAMESPACE" help:"Kubernetes namespace for UXP." short:"n"`

	// Common Upbound API configuration
	Flags upbound.Flags `embed:""`
}
