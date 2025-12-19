// Copyright 2025 Upbound Inc.
// All rights reserved

package migration

import (
	"context"

	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	"k8s.io/client-go/restmapper"

	"github.com/upbound/up/internal/input"
	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/pkg/migration"
	"github.com/upbound/up/pkg/migration/exporter"

	_ "embed"
)

const secretsWarning = `Warning: A functional Crossplane control plane requires cloud provider credentials,
which are stored as Kubernetes secrets. Additionally, some managed resources provide
connection details exclusively during provisioning, and these details may not be
reconstructable post-migration. Consequently, the exported archive will incorporate
those secrets by default. To exclude secrets from the export, please use the
--exclude-resources flag.

IMPORTANT: The exported archive will contain secrets. Do you wish to proceed?`

type exportCmd struct {
	prompter input.Prompter

	Yes bool `default:"false" help:"When set to true, automatically accepts any confirmation prompts that may appear during the export process."`

	Output string `default:"xp-state.tar.gz" help:"Specifies the file path where the exported archive will be saved. Defaults to 'xp-state.tar.gz'." short:"o"`

	IncludeExtraResources []string `default:"namespaces,configmaps,secrets"                                                                                        help:"A list of extra resource types to include in the export in \"resource.group\" format in addition to all Crossplane resources. By default, it includes namespaces, configmaps, secrets."`
	ExcludeResources      []string `help:"A list of resource types to exclude from the export in \"resource.group\" format. No resources are excluded by default."`
	IncludeNamespaces     []string `help:"A list of specific namespaces to include in the export. If not specified, all namespaces are included by default."`
	ExcludeNamespaces     []string `default:"kube-system,kube-public,kube-node-lease,local-path-storage"                                                           help:"A list of specific namespaces to exclude from the export. Defaults to 'kube-system', 'kube-public', 'kube-node-lease', and 'local-path-storage'."`

	PauseBeforeExport bool `default:"false" help:"When set to true, pauses all claim,composite and managed resources before starting the export process. This can help ensure a consistent state for the export. Defaults to false."`
}

//go:embed help/export.md
var exportHelp string

func (c *exportCmd) Help() string {
	return exportHelp
}

// BeforeApply sets default values for the delete command, before assignment and validation.
func (c *exportCmd) BeforeApply() error {
	c.prompter = input.NewPrompter()
	return nil
}

func (c *exportCmd) Run(ctx context.Context, migCtx *migration.Context, printer upterm.Printer) error {
	cfg := migCtx.Kubeconfig

	crdClient, err := apiextensionsclientset.NewForConfig(cfg)
	if err != nil {
		return err
	}
	dynamicClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return err
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return err
	}
	appsClient, err := appsv1.NewForConfig(cfg)
	if err != nil {
		return err
	}

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(discoveryClient))

	e := exporter.NewControlPlaneStateExporter(crdClient, dynamicClient, discoveryClient, appsClient, mapper, exporter.Options{
		OutputArchive: c.Output,

		IncludeNamespaces:     c.IncludeNamespaces,
		ExcludeNamespaces:     c.ExcludeNamespaces,
		IncludeExtraResources: c.IncludeExtraResources,
		ExcludeResources:      c.ExcludeResources,

		PauseBeforeExport: c.PauseBeforeExport,
	})

	if !c.Yes && e.IncludedExtraResource("secrets") {
		result, _ := upterm.Confirm(secretsWarning, true)
		if !result {
			return nil
		}
	}

	printer.Println("Exporting control plane state...")
	migration.DefaultSpinner = func(msg string) migration.Spinner { return printer.NewSuccessSpinner(msg) }

	if err = e.Export(ctx); err != nil {
		return err
	}
	printer.Println("\nSuccessfully exported control plane state!")
	return nil
}
