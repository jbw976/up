// Copyright 2025 Upbound Inc.
// All rights reserved

package uxp

import (
	"net/url"

	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/install/helm"
	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/internal/uxp"
)

// AfterApply sets default values in command after assignment and validation.
func (c *uninstallCmd) AfterApply(insCtx *install.Context) error {
	// NOTE(hasheddan): we always pass default repo URL because the repo URL is
	// not considered during uninstall.
	mgr, err := helm.NewManager(insCtx.Kubeconfig,
		uxp.ChartName,
		url.URL{},
		uxp.ChartNamespace,
		helm.Wait(),
	)
	if err != nil {
		return err
	}
	c.mgr = mgr
	return nil
}

// uninstallCmd uninstalls UXP.
type uninstallCmd struct {
	mgr install.Manager
}

// Run executes the uninstall command.
func (c *uninstallCmd) Run(p upterm.Printer) error {
	if err := c.mgr.Uninstall(); err != nil {
		return err
	}
	p.Printfln("UXP uninstalled")
	return nil
}
