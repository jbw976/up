// Copyright 2025 Upbound Inc.
// All rights reserved

package webui

import (
	"github.com/pterm/pterm"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/install/helm"
	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/internal/uxp"
)

const (
	errDisableWebUI = "failed to disable web UI"
)

// disableCmd disables the UXP web UI.
type disableCmd struct {
	Unstable bool `help:"Allow upgrading unstable chart versions."`
}

func (c *disableCmd) Run(insCtx *install.Context, p upterm.ObjectPrinter) error {
	repo := uxp.RepoURL
	if c.Unstable {
		repo = uxp.UnstableRepoURL
	}
	mgr, err := helm.NewManager(insCtx.Kubeconfig,
		uxp.ChartName,
		*repo,
		uxp.ChartNamespace,
		helm.UpgradeReuseValues(),
		helm.Wait(),
	)
	if err != nil {
		return errors.Wrap(err, errCreateHelmManager)
	}

	values := map[string]any{
		"webui": map[string]any{
			"enabled": false,
		},
	}

	if err := upterm.WrapWithSuccessSpinner(
		"Disabling UXP web UI",
		upterm.CheckmarkSuccessSpinner,
		func() error {
			currentVersion, err := mgr.GetCurrentVersion()
			if err != nil {
				return errors.Wrap(err, errGetVersion)
			}
			return errors.Wrap(mgr.Upgrade(currentVersion, values), errDisableWebUI)
		},
		p,
	); err != nil {
		return err
	}
	pterm.Info.WithPrefix(upterm.RaisedPrefix).Println("UXP web UI disabled")
	return nil
}
