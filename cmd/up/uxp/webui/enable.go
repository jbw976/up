// Copyright 2025 Upbound Inc.
// All rights reserved

package webui

import (
	"github.com/pterm/pterm"

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

func (c *enableCmd) Run(insCtx *install.Context, p upterm.ObjectPrinter) error {
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

	if err := upterm.WrapWithSuccessSpinner(
		"Enabling UXP web UI",
		upterm.CheckmarkSuccessSpinner,
		func() error {
			currentVersion, err := mgr.GetCurrentVersion()
			if err != nil {
				return errors.Wrap(err, errGetVersion)
			}
			return errors.Wrap(mgr.Upgrade(currentVersion, values), errEnableWebUI)
		},
		p,
	); err != nil {
		return err
	}
	pterm.Info.WithPrefix(upterm.RaisedPrefix).Println("UXP web UI enabled")
	return nil
}
