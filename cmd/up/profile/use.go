// Copyright 2025 Upbound Inc.
// All rights reserved

package profile

import (
	"context"

	"github.com/pterm/pterm"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	ctxcmd "github.com/upbound/up/cmd/up/ctx"
	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/style"
	"github.com/upbound/up/internal/upbound"
)

const (
	errUpdateProfile = "unable to update profile"
)

type useCmd struct {
	Name string `arg:"" help:"Name of the Profile to use." predictor:"profiles" required:""`
}

func (c *useCmd) Help() string {
	return style.RenderHelp(`
The <use> command switches the active Upbound profile and updates the kubeconfig context.

This command:
- Sets the specified profile as the default profile
- Updates the kubeconfig to use the context associated with the profile
- Preserves any existing kubeconfig context information from the profile

## Usage Examples:

    up profile use <production>
        Switches to the "production" profile and updates the kubeconfig context.

    up profile use <dev>
        Switches to the "dev" profile and updates the kubeconfig context.

Note: If the profile has no associated kubeconfig context, only the profile switch
occurs without kubeconfig updates.
`)
}

// Run executes the Use command.
func (c *useCmd) Run(ctx context.Context, upCtx *upbound.Context, flags upbound.Flags, p pterm.TextPrinter) error {
	if err := upCtx.Cfg.SetDefaultUpboundProfile(c.Name); err != nil {
		return err
	}

	if err := upCtx.CfgSrc.UpdateConfig(upCtx.Cfg); err != nil {
		return errors.Wrap(err, errUpdateProfile)
	}

	p.Printfln("Using profile %q", c.Name)

	// Create a new upCtx with the new profile active.
	flags.Profile = c.Name
	upCtx, err := upbound.NewFromFlags(flags)
	if err != nil {
		return err
	}

	contextPath := upCtx.Profile.CurrentKubeContext
	if contextPath == "" {
		// This profile never had a kube context recorded, so don't update the
		// kubeconfig.
		return nil
	}

	if err := setKubeconfigContext(ctx, upCtx, flags.Kube); err != nil {
		return err
	}

	p.Printfln("Selected Upbound kubeconfig context %q", upCtx.Profile.CurrentKubeContext)

	return nil
}

func setKubeconfigContext(ctx context.Context, upCtx *upbound.Context, flags upbound.KubeFlags) error {
	// Get a kubeconfig for the context stored in the profile.
	conf, err := ctxcmd.GetKubeconfigForPath(ctx, upCtx, upCtx.Profile.CurrentKubeContext)
	if err != nil {
		return errors.Wrap(err, "failed to get kubeconfig for profile's context")
	}

	contextName := flags.Context
	if contextName == "" {
		contextName = "upbound"
	}

	wr := kube.NewFileWriter(upCtx, flags.Kubeconfig, contextName)
	if err := wr.Write(conf); err != nil {
		return errors.Wrap(err, "failed to write kubeconfig for profile's context")
	}

	return nil
}
