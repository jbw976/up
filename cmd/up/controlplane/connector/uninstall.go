// Copyright 2025 Upbound Inc.
// All rights reserved

package connector

import (
	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"

	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/install/helm"
	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/upbound"
)

// AfterApply sets default values in command after assignment and validation.
func (c *uninstallCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	if c.ClusterName == "" {
		c.ClusterName = c.Namespace
	}
	kubeconfig, err := kube.GetKubeConfig(c.Kubeconfig)
	if err != nil {
		return err
	}

	mgr, err := helm.NewManager(kubeconfig,
		connectorName,
		mcpRepoURL,
		helm.WithNamespace(c.InstallationNamespace),
		helm.IsOCI(),
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
	Kubeconfig            string `help:"Override the default kubeconfig path."                                                                            type:"existingfile"`
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
