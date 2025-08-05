// Copyright 2025 Upbound Inc.
// All rights reserved

package connector

import (
	"github.com/pterm/pterm"

	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/install/helm"
	"github.com/upbound/up/internal/upbound"
)

// AfterApply sets default values in command after assignment and validation.
func (c *uninstallCmd) AfterApply(upCtx *upbound.Context) error {
	if c.ClusterName == "" {
		c.ClusterName = c.Namespace
	}
	kubeconfig, err := upCtx.GetKubeconfig()
	if err != nil {
		return err
	}

	mgr, err := helm.NewManager(kubeconfig,
		connectorName,
		mcpRepoURL,
		c.InstallationNamespace,
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

	ClusterName           string `help:"Name of the cluster connecting to the control plane. If not provided, the namespace argument value will be used."`
	Namespace             string `arg:""                                                                                                                  help:"Namespace in the control plane where the claims of the cluster will be stored." required:""`
	InstallationNamespace string `default:"kube-system"                                                                                                   env:"MCP_CONNECTOR_NAMESPACE"                                                         help:"Kubernetes namespace for MCP Connector. Default is kube-system." short:"n"`
}

// Run executes the uninstall command.
func (c *uninstallCmd) Run(p pterm.TextPrinter) error {
	if err := c.mgr.Uninstall(); err != nil {
		return err
	}
	p.Printfln("MCP Connector uninstalled")
	return nil
}
