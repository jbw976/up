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

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/upbound/up/internal/input"
	"github.com/upbound/up/internal/profile"
	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/pkg/migration"
	"github.com/upbound/up/pkg/migration/importer"

	_ "embed"
)

type importCmd struct {
	prompter input.Prompter
	Yes      bool `default:"false" help:"When set to true, automatically accepts any confirmation prompts that may appear during the import process."`

	Input string `default:"xp-state.tar.gz" help:"Specifies the file path or directory of the archive to be imported. The default path is 'xp-state.tar.gz'." short:"i"`

	UnpauseAfterImport bool `default:"false" help:"When set to true, automatically unpauses all managed resources that were paused during the import process. This helps in resuming normal operations post-import. Defaults to false, requiring manual unpausing of resources if needed."`

	// MCPConnectorClusterID specifies the MCP Connector cluster ID.
	// https://github.com/upbound/mcp-connector/blob/b8a55b698d5d0c1343faf53110738f9bb1865705/cluster/charts/mcp-connector/values.yaml.tmpl#L11
	MCPConnectorClusterID string `help:"MCP Connector cluster ID. Required for importing claims supported my MCP Connector."`

	// MCPConnectorClaimNamespace defines the MCP Connector claim namespace.
	// https://github.com/upbound/mcp-connector/blob/b8a55b698d5d0c1343faf53110738f9bb1865705/cluster/charts/mcp-connector/values.yaml.tmpl#L49
	MCPConnectorClaimNamespace string `help:"MCP Connector claim namespace. Required for importing claims supported by MCP Connector."`

	SkipTargetCheck bool `default:"false" help:"When set to true, skips the check for a local or managed control plane during import." hidden:""`
}

//go:embed help/import.md
var importHelp string

func (c *importCmd) Help() string {
	return importHelp
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
	})

	errs := i.PreflightChecks(ctx)
	if len(errs) > 0 {
		pterm.Println("Preflight checks failed:")
		for _, err := range errs {
			pterm.Println("- " + err.Error())
		}
		if !c.Yes {
			result, _ := upterm.Confirm("Do you still want to proceed?", false)
			if !result {
				pterm.Error.Println("Preflight checks must pass in order to proceed with the import.")
				return nil
			}
		}
	}

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
