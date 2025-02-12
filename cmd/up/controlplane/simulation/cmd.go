// Copyright 2025 Upbound Inc.
// All rights reserved

// Package simulation contains commands for working with control plane
// simulations.
package simulation

import (
	"time"

	"github.com/alecthomas/kong"
	"k8s.io/apimachinery/pkg/util/duration"
	kruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	spacesv1alpha1 "github.com/upbound/up-sdk-go/apis/spaces/v1alpha1"
	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	"github.com/upbound/up/internal/feature"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

func init() {
	kruntime.Must(spacesv1alpha1.AddToScheme(scheme.Scheme))
	kruntime.Must(spacesv1beta1.AddToScheme(scheme.Scheme))
}

// BeforeReset is the first hook to run.
func (c *Cmd) BeforeReset(p *kong.Path, maturity feature.Maturity) error {
	return feature.HideMaturity(p, maturity)
}

// AfterApply constructs and binds a control plane client to any subcommands
// that have Run() methods that receive it.
func (c *Cmd) AfterApply(kongCtx *kong.Context) error {
	upCtx, err := upbound.NewFromFlags(c.Flags)
	if err != nil {
		return err
	}
	kongCtx.Bind(upCtx)

	// we can't use control planes from inside a control plane
	if _, ctp, isSpace := upCtx.GetCurrentSpaceContextScope(); isSpace && ctp.Name != "" {
		return errors.New("cannot access simulations from inside a control plane context. Use 'up ctx ..' to go up to the group context")
	}

	cl, err := upCtx.BuildCurrentContextClient()
	if err != nil {
		return errors.Wrap(err, "unable to get kube client")
	}
	kongCtx.BindTo(cl, (*client.Client)(nil))

	return nil
}

// Cmd contains commands for interacting with control planes.
type Cmd struct {
	Create CreateCmd `cmd:"" help:"Start a new control plane simulation and wait for the results."`
	Delete deleteCmd `cmd:"" help:"Delete a control plane simulation."`
	List   listCmd   `cmd:"" help:"List control plane simulations for the account."`

	// Common Upbound API configuration
	Flags upbound.Flags `embed:""`
}

// Help prints help.
func (c *Cmd) Help() string {
	return `
Manage control plane simulations. Simulations allow you to "simulate" what
happens on the control plane and see what would changes after the changes are
applied.
`
}

func extractFields(obj any) []string {
	sim, ok := obj.(spacesv1alpha1.Simulation)
	if !ok {
		return []string{"unknown", "unknown", "", "", "", "", ""}
	}

	simulated := ""
	if sim.Status.SimulatedControlPlaneName != nil {
		simulated = *sim.Status.SimulatedControlPlaneName
	}

	return []string{
		sim.GetNamespace(),
		sim.GetName(),
		sim.Spec.ControlPlaneName,
		simulated,
		string(sim.Status.GetCondition(spacesv1alpha1.TypeAcceptingChanges).Status),
		string(sim.Status.GetCondition(spacesv1alpha1.TypeAcceptingChanges).Reason),
		formatAge(ptr.To(time.Since(sim.CreationTimestamp.Time))),
	}
}

func formatAge(age *time.Duration) string {
	if age == nil {
		return ""
	}

	return duration.HumanDuration(*age)
}

func tabularPrint(obj any, printer upterm.ObjectPrinter) error {
	fieldNames := []string{"GROUP", "NAME", "SOURCE", "SIMULATED", "ACCEPTING-CHANGES", "STATE", "AGE"}
	return printer.Print(obj, fieldNames, extractFields)
}
