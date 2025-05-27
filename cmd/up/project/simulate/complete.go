// Copyright 2025 Upbound Inc.
// All rights reserved

// Package simulate provides the `up project simulate` command.
package simulate

import (
	"context"

	"github.com/alecthomas/kong"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/scheme"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	xpkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	xpkgv1beta1 "github.com/crossplane/crossplane/apis/pkg/v1beta1"

	"github.com/upbound/up/internal/async"
	"github.com/upbound/up/internal/config"
	intctx "github.com/upbound/up/internal/ctx"
	"github.com/upbound/up/internal/simulation"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

// completeCmd is the `up project simulation complete` command.
type completeCmd struct {
	ProjectFile string `default:"upbound.yaml" help:"Path to project definition file."    short:"f"`
	Name        string `arg:""                 help:"The name of the simulation resource"`

	Output            string `help:"Output the results of the simulation to the provided file. Defaults to standard out if not specified" short:"o"`
	TerminateOnFinish bool   `default:"true"                                                                                              help:"Terminate the simulation after the completion criteria is met"`

	ControlPlaneGroup string        `help:"The control plane group that the control plane to use is contained in. This defaults to the group specified in the current context." short:"g"`
	GlobalFlags       upbound.Flags `embed:""`

	spaceClient client.Client

	quiet        config.QuietFlag
	asyncWrapper async.WrapperFunc
}

// AfterApply processes flags and sets defaults.
func (c *completeCmd) AfterApply(kongCtx *kong.Context, printer upterm.ObjectPrinter) error {
	upCtx, err := upbound.NewFromFlags(c.GlobalFlags)
	if err != nil {
		return err
	}
	upCtx.SetupLogging()
	kongCtx.Bind(upCtx)

	spaceClientConfig, err := intctx.GetSpacesKubeconfig(context.Background(), upCtx)
	if err != nil {
		return errors.Wrap(err, "cannot get spaces kubeconfig")
	}
	spaceClientREST, err := spaceClientConfig.ClientConfig()
	if err != nil {
		return errors.Wrap(err, "failed to get REST config for space client")
	}
	c.spaceClient, err = client.New(spaceClientREST, client.Options{})
	if err != nil {
		return err
	}

	if c.ControlPlaneGroup == "" {
		ns, _, err := spaceClientConfig.Namespace()
		if err != nil {
			return err
		}
		c.ControlPlaneGroup = ns
	}

	c.quiet = printer.Quiet
	switch {
	case bool(printer.Quiet):
		c.asyncWrapper = async.IgnoreEvents
	case printer.Pretty:
		c.asyncWrapper = async.WrapWithSuccessSpinnersPretty
	default:
		c.asyncWrapper = async.WrapWithSuccessSpinnersNonPretty
	}
	return nil
}

// Run is the body of the command.
func (c *completeCmd) Run(ctx context.Context, upCtx *upbound.Context, kongCtx *kong.Context) error {
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

	if err := sim.Complete(ctx, c.spaceClient); err != nil {
		return err
	}

	err = c.asyncWrapper(func(ch async.EventChannel) error {
		stageStatus := "Waiting for Simulation to complete"
		ch.SendEvent(stageStatus, async.EventStatusStarted)
		err := sim.WaitForCondition(ctx, c.spaceClient, simulation.Complete())
		if err != nil {
			ch.SendEvent(stageStatus, async.EventStatusFailure)
		} else {
			ch.SendEvent(stageStatus, async.EventStatusSuccess)
		}
		return err
	})
	if err != nil {
		return err
	}

	diffSet, err := sim.DiffSet(ctx, upCtx, []schema.GroupKind{
		xpkgv1.ConfigurationGroupVersionKind.GroupKind(),
	})
	if err != nil {
		return err
	}

	if err := outputDiff(kongCtx, diffSet, c.Output); err != nil {
		return err
	}

	if c.TerminateOnFinish {
		if err := sim.Terminate(ctx, c.spaceClient); err != nil {
			return err
		}
	}

	return nil
}
