// Copyright 2025 Upbound Inc.
// All rights reserved

package webui

import (
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/install/helm"
	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/internal/uxp"
)

const (
	errCreateHelmManager = "failed to create helm manager"
	errEnableWebUI       = "failed to enable web UI"
	errGetVersion        = "failed to get current UXP version"
)

// enableCmd enables the UXP web UI.
type enableCmd struct {
	Unstable bool `help:"Allow upgrading unstable chart versions."`
}

func (c *enableCmd) Run(insCtx *install.Context, p upterm.Printer) error {
	repo := uxp.RepoURL

	filter := uxp.StableVersionFilter
	if c.Unstable {
		filter = uxp.UnstableVersionFilter
	}

	mgr, err := helm.NewManager(insCtx.Kubeconfig,
		uxp.ChartName,
		*repo,
		uxp.ChartNamespace,
		helm.UpgradeReuseValues(),
		helm.WithVersionFilter(filter),
		helm.Wait(),
	)
	if err != nil {
		return errors.Wrap(err, errCreateHelmManager)
	}

	values := map[string]any{
		"webui": map[string]any{
			"enabled": true,
		},
	}

	if err := p.WrapWithSuccessSpinner(
		"Enabling UXP web UI",
		func() error {
			currentVersion, err := mgr.GetCurrentVersion()
			if err != nil {
				return errors.Wrap(err, errGetVersion)
			}
			return errors.Wrap(mgr.Upgrade(currentVersion, values), errEnableWebUI)
		},
	); err != nil {
		return err
	}
	p.PrintSuccess("UXP web UI enabled")
	return nil
}
