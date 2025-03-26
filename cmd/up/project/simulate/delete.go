// Copyright 2025 Upbound Inc.
// All rights reserved

package simulate

import (
	"context"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/scheme"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	xpkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	xpkgv1beta1 "github.com/crossplane/crossplane/apis/pkg/v1beta1"

	ctxcmd "github.com/upbound/up/cmd/up/ctx"
	"github.com/upbound/up/internal/async"
	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/ctx"
	"github.com/upbound/up/internal/simulation"
	"github.com/upbound/up/internal/upbound"
)

// deleteCmd is the `up project simulation delete` command.
type deleteCmd struct {
	ProjectFile string `default:"upbound.yaml" help:"Path to project definition file."    short:"f"`
	Name        string `arg:""                 help:"The name of the simulation resource"`

	ControlPlaneGroup string        `help:"The control plane group that the control plane to use is contained in. This defaults to the group specified in the current context." short:"g"`
	GlobalFlags       upbound.Flags `embed:""`

	spaceClient client.Client

	quiet        config.QuietFlag
	asyncWrapper async.WrapperFunc
}

// AfterApply processes flags and sets defaults.
func (c *deleteCmd) AfterApply(kongCtx *kong.Context, quiet config.QuietFlag) error {
	upCtx, err := upbound.NewFromFlags(c.GlobalFlags)
	if err != nil {
		return err
	}
	upCtx.SetupLogging()
	kongCtx.Bind(upCtx)

	spaceCtx, err := ctx.GetCurrentSpaceNavigation(context.Background(), upCtx)
	if err != nil {
		return err
	}

	var ok bool
	var space ctxcmd.Space

	if space, ok = spaceCtx.(ctxcmd.Space); !ok {
		if group, ok := spaceCtx.(*ctxcmd.Group); ok {
			space = group.Space
			if c.ControlPlaneGroup == "" {
				c.ControlPlaneGroup = group.Name
			}
		} else if ctp, ok := spaceCtx.(*ctxcmd.ControlPlane); ok {
			space = ctp.Group.Space
			if c.ControlPlaneGroup == "" {
				c.ControlPlaneGroup = ctp.Group.Name
			}
		} else {
			return errors.New("current kubeconfig is not pointed at an Upbound Cloud Space; use `up ctx` to select a Space")
		}
	}

	// fallback to the default "default" group
	if c.ControlPlaneGroup == "" {
		c.ControlPlaneGroup = "default"
	}

	// Get the client for parent space, even if pointed at a group or a control
	// plane
	spaceClientConfig, err := space.BuildKubeconfig(types.NamespacedName{
		Namespace: c.ControlPlaneGroup,
	})
	if err != nil {
		return errors.Wrap(err, "failed to build space client")
	}
	spaceClientREST, err := spaceClientConfig.ClientConfig()
	if err != nil {
		return errors.Wrap(err, "failed to get REST config for space client")
	}
	c.spaceClient, err = client.New(spaceClientREST, client.Options{})
	if err != nil {
		return err
	}

	pterm.EnableStyling()

	c.quiet = quiet
	c.asyncWrapper = async.WrapWithSuccessSpinners
	if quiet {
		c.asyncWrapper = async.IgnoreEvents
	}

	return nil
}

// Run is the body of the command.
func (c *deleteCmd) Run(ctx context.Context, upCtx *upbound.Context) error {
	sim, err := simulation.GetExisting(ctx, c.spaceClient, types.NamespacedName{
		Namespace: c.ControlPlaneGroup,
		Name:      c.Name,
	})
	if err != nil {
		return err
	}

	simConfig, err := sim.RESTConfig(ctx, upCtx)
	if err != nil {
		return errors.Wrap(err, "failed to get simulated control plane kubeconfig")
	}
	simClient, err := client.New(simConfig, client.Options{})
	if err != nil {
		return errors.Wrap(err, "failed to build simulated control plane client")
	}

	ctpSchemeBuilders := []*scheme.Builder{
		xpkgv1.SchemeBuilder,
		xpkgv1beta1.SchemeBuilder,
	}
	for _, bld := range ctpSchemeBuilders {
		if err := bld.AddToScheme(simClient.Scheme()); err != nil {
			return err
		}
	}

	if err := sim.Delete(ctx, c.spaceClient); err != nil {
		return err
	}

	return nil
}
