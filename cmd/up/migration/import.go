// Copyright 2025 Upbound Inc.
// All rights reserved

// Package migration contains functions for migration
package migration

import (
	"context"

	"github.com/pterm/pterm"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	"k8s.io/client-go/restmapper"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/input"
	"github.com/upbound/up/internal/profile"
	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/pkg/migration"
	"github.com/upbound/up/pkg/migration/importer"
)

type importCmd struct {
	prompter input.Prompter
	Yes      bool `default:"false" help:"When set to true, automatically accepts any confirmation prompts that may appear during the import process."`

	Input string `default:"xp-state.tar.gz" help:"Specifies the file path of the archive to be imported. The default path is 'xp-state.tar.gz'." short:"i"`

	UnpauseAfterImport bool `default:"false" help:"When set to true, automatically unpauses all managed resources that were paused during the import process. This helps in resuming normal operations post-import. Defaults to false, requiring manual unpausing of resources if needed."`

	// MCPConnectorClusterID specifies the MCP Connector cluster ID.
	// https://github.com/upbound/mcp-connector/blob/b8a55b698d5d0c1343faf53110738f9bb1865705/cluster/charts/mcp-connector/values.yaml.tmpl#L11
	MCPConnectorClusterID string `help:"MCP Connector cluster ID. Required for importing claims supported my MCP Connector."`

	// MCPConnectorClaimNamespace defines the MCP Connector claim namespace.
	// https://github.com/upbound/mcp-connector/blob/b8a55b698d5d0c1343faf53110738f9bb1865705/cluster/charts/mcp-connector/values.yaml.tmpl#L49
	MCPConnectorClaimNamespace string `help:"MCP Connector claim namespace. Required for importing claims supported by MCP Connector."`

	ImportClaimsOnly bool `default:"false" help:"When set to true, only Claims will be imported"`

	SkipTargetCheck bool `default:"false" help:"When set to true, skips the check for a local or managed control plane during import." hidden:""`
}

func (c *importCmd) Help() string {
	return `
By default, all managed resources will be paused during the import process for possible manual inspection/validation.
You can use the --unpause-after-import flag to automatically unpause all claim,composite,managed resources after the import process completes.

Examples:
    migration import --input=my-export.tar.gz
        Automatically imports the control plane state from my-export.tar.gz.
        Claim and composite resources that were paused during export will remain paused.
        Managed resources will be paused. If they were already paused during export, the annotation migration.upbound.io/already-paused: "true" will be added to preserve their paused state.

    migration import --unpause-after-import
        Automatically imports and unpauses claim,composite,managed resources after the import.
		Resources with the annotation migration.upbound.io/already-paused: "true" will remain paused.

	migration import --unpause-after-import --mcp-connector-claim-namespace=default --mcp-connector-cluster-id=my-cluster-id
		Automatically imports and unpauses claims, composites, and managed resources after the import process.
		The metadata.name of claims will be adjusted for MCP Connector compatibility, and the corresponding composite's claimRef will also be updated.
		Resources annotated with migration.upbound.io/already-paused: "true" will remain paused.
`
}

// BeforeApply sets default values for the delete command, before assignment and validation.
func (c *importCmd) BeforeApply() error {
	c.prompter = input.NewPrompter()

	if (c.MCPConnectorClaimNamespace == "") != (c.MCPConnectorClusterID == "") {
		return errors.New("both MCPConnectorClaimNamespace and MCPConnectorClusterID must be set or both must be empty")
	}

	return nil
}

func (c *importCmd) Run(ctx context.Context, migCtx *migration.Context) error { //nolint:gocyclo // Just a lot of error handling.
	cfg := migCtx.Kubeconfig

	if !c.SkipTargetCheck && !isAllowedImportTarget(cfg.Host) {
		return errors.New("not a local or managed control plane, import not supported")
	}

	dynamicClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return err
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return err
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(discoveryClient))

	appsClient, err := appsv1.NewForConfig(cfg)
	if err != nil {
		return err
	}

	i := importer.NewControlPlaneStateImporter(dynamicClient, discoveryClient, appsClient, mapper, importer.Options{
		InputArchive: c.Input,

		UnpauseAfterImport: c.UnpauseAfterImport,

		MCPConnectorClusterID:      c.MCPConnectorClusterID,
		MCPConnectorClaimNamespace: c.MCPConnectorClaimNamespace,
		ImportClaimsOnly:           c.ImportClaimsOnly,
	})

	if !c.ImportClaimsOnly {
		errs := i.PreflightChecks(ctx)
		if len(errs) > 0 {
			pterm.Println("Preflight checks failed:")
			for _, err := range errs {
				pterm.Println("- " + err.Error())
			}
			if !c.Yes {
				pterm.Println() // Blank line
				confirm := pterm.DefaultInteractiveConfirm
				confirm.DefaultText = "Do you still want to proceed?"
				confirm.DefaultValue = false
				result, _ := confirm.Show()
				pterm.Println() // Blank line
				if !result {
					pterm.Error.Println("Preflight checks must pass in order to proceed with the import.")
					return nil
				}
			}
		}
	}

	pterm.EnableStyling()
	upterm.DefaultObjPrinter.Pretty = true

	pterm.Println("Importing control plane state...")
	migration.DefaultSpinner = &spinner{upterm.CheckmarkSuccessSpinner}

	if err = i.Import(ctx); err != nil {
		return err
	}
	pterm.Println("\nfully imported control plane state!")

	return nil
}

func isAllowedImportTarget(host string) bool {
	_, matches := profile.ParseMCPK8sURL(host)
	if !matches {
		_, _, matches = profile.ParseSpacesK8sURL(host)
	}
	if !matches {
		matches = profile.ParseLocalHostURL(host)
	}
	return matches
}
