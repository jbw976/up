// Copyright 2025 Upbound Inc.
// All rights reserved

package apiconnector

import (
	"context"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up-sdk-go"
	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

func (c *uninstallCmd) Help() string {
	return `
The 'uninstall' command uninstalls the API Connector from a cluster.

Examples:
    up controlplane api-connector uninstall --target-kubeconfig <kubeconfig-path-for-deployment-cluster>
	    Uninstalls the API Connector from the cluster but leaves the connections and secrets.

    up controlplane api-connector uninstall --all --target-kubeconfig <kubeconfig-path-for-deployment-cluster>
        Uninstalls the API Connector from the cluster and deletes the connections and secrets.
		It will not delete API objects created by the API Connector initial installation.
`
}

// AfterApply sets default values in command after assignment and validation.
func (c *uninstallCmd) AfterApply(_ *kong.Context, upCtx *upbound.Context) error {
	var consumerRestConfig *rest.Config
	var err error
	consumerRestConfig, err = kube.GetKubeConfigWithContext(c.ConsumerKubeconfig, c.ConsumerContext)
	if err != nil {
		return err
	}

	if err != nil {
		return err
	}
	c.targetRestConfig = consumerRestConfig

	targetKubeClient, err := client.New(consumerRestConfig, client.Options{
		Scheme: scheme.Scheme,
	})
	if err != nil {
		return err
	}
	c.targetClient = targetKubeClient

	cfg, err := upCtx.BuildSDKConfig()
	if err != nil {
		return errors.Wrap(err, "failed to build SDK config")
	}
	c.sdkConfig = cfg

	return nil
}

// uninstallCmd uninstalls API Connector.
type uninstallCmd struct {
	sdkConfig        *up.Config
	targetClient     client.Client
	targetRestConfig *rest.Config

	ConsumerKubeconfig string `help:"Path to the kubeconfig file for the consumer cluster. If not provided, the default kubeconfig resolution will be used." required:"true"`
	ConsumerContext    string `help:"Context to use in the kubeconfig file. If not provided, the current context will be used."`

	All bool `help:"Uninstall all resources including the connectors and secrets. If not provided, only the connector will be uninstalled."`
}

// Run executes the uninstall command.
func (c *uninstallCmd) Run(p pterm.TextPrinter, printer upterm.ObjectPrinter) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	provisioner := newProvisioner(c.sdkConfig, p, printer)

	err := provisioner.uninstallConnector(ctx, c.targetRestConfig, installOptions{
		namespace: defaultInstallationNamespace,
	})
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return errors.Wrap(err, "failed to uninstall connector")
	}

	if c.All {
		err := provisioner.deleteConnectionSecrets(ctx, c.targetClient, defaultInstallationNamespace)
		if err != nil {
			return errors.Wrap(err, "failed to delete connection secret")
		}

		err = provisioner.deleteConnections(ctx, c.targetClient)
		if err != nil {
			return errors.Wrap(err, "failed to delete connections")
		}
	}

	p.Printfln("API Connector uninstalled")
	return nil
}
