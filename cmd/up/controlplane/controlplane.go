// Copyright 2025 Upbound Inc.
// All rights reserved

// Package controlplane contains functions for handling controlplane actions
package controlplane

import (
	"context"
	"time"

	"github.com/alecthomas/kong"
	"github.com/posener/complete"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpcommonv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	apiconnector "github.com/upbound/up/cmd/up/controlplane/api-connector"
	"github.com/upbound/up/cmd/up/controlplane/connector"
	"github.com/upbound/up/cmd/up/controlplane/oidcauth"
	"github.com/upbound/up/cmd/up/controlplane/pkg"
	"github.com/upbound/up/cmd/up/controlplane/pullsecret"
	"github.com/upbound/up/cmd/up/controlplane/requires"
	"github.com/upbound/up/cmd/up/controlplane/simulation"
	"github.com/upbound/up/cmd/up/migration"
	"github.com/upbound/up/internal/feature"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

// BeforeReset is the first hook to run.
func (c *Cmd) BeforeReset(p *kong.Path, maturity feature.Maturity) error {
	return feature.HideMaturity(p, maturity)
}

// AfterApply constructs and binds a k8s client to any subcommands that have
// Run() methods that receive it.
func (c *Cmd) AfterApply(kongCtx *kong.Context) error {
	upCtx, err := upbound.NewFromFlags(c.Flags)
	if err != nil {
		return err
	}
	upCtx.SetupLogging()
	kongCtx.Bind(upCtx)

	// Check that the current context meets the command's requirements and
	// construct a kube context. The requirement may be on a parent node, so
	// iterate up the command tree.
	for current := kongCtx.Selected(); current != nil; current = current.Parent {
		if req, ok := current.Target.Interface().(requires.Checker); ok {
			cl, err := req.Check(context.Background(), upCtx)
			if err != nil {
				return err
			}
			kongCtx.BindTo(cl, (*client.Client)(nil))
			break
		}
	}

	return nil
}

// PredictControlPlanes provides a predictor for control planes.
// This function is used by the kongplete.Complete package for shell autocompletion.
func PredictControlPlanes() complete.Predictor {
	return complete.PredictFunc(func(_ complete.Args) (prediction []string) {
		upCtx, err := upbound.NewFromFlags(upbound.Flags{})
		if err != nil {
			return nil
		}
		upCtx.SetupLogging()

		client, err := upCtx.BuildCurrentContextClient()
		if err != nil {
			return nil
		}

		var l spacesv1beta1.ControlPlaneList
		if err := client.List(context.Background(), &l); err != nil {
			return nil
		}

		if len(l.Items) == 0 {
			return nil
		}

		data := make([]string, len(l.Items))
		for i, ctp := range l.Items {
			data[i] = ctp.Name
		}
		return data
	})
}

// Cmd contains commands for interacting with control planes.
//
// Each subcommand struct must embed a struct from the `requires` package to
// indicate what kind of kube context it needs.
type Cmd struct {
	// Commands for managing control planes in Spaces. These require a space
	// context.
	Create createCmd `cmd:"" help:"Create a Spaces control plane."`
	Delete deleteCmd `cmd:"" help:"Delete a Spaces control plane."`
	List   listCmd   `cmd:"" help:"List control planes in a Space."`
	Get    getCmd    `cmd:"" help:"Get a single Spaces control plane."`

	// Commands for managing the connector. These require a control plane
	// context.
	Connector connector.Cmd `cmd:"" help:"Connect an App Cluster to a control plane using MCP Connector."`
	// Command for managing the api connectors.These require a space
	// context.
	APIConnector apiconnector.Cmd `cmd:"" help:"Connect an App Cluster to a control plane using API Connector."`

	// Commands for managing control plane simulations. These require a space
	// context.
	Simulation simulation.Cmd       `aliases:"sim" cmd:""                                                help:"Manage control plane simulations." maturity:"alpha"`
	Simulate   simulation.CreateCmd `cmd:""        help:"Alias for 'up controlplane simulation create'." maturity:"alpha"`

	// Commands for managing packages in control planes. These require a control
	// plane context.
	Configuration pkg.Cmd `cmd:"" help:"Manage Configurations." set:"package_type=Configuration"`
	Provider      pkg.Cmd `cmd:"" help:"Manage Providers."      set:"package_type=Provider"`
	Function      pkg.Cmd `cmd:"" help:"Manage Functions."      set:"package_type=Function"`

	// Commands for managing pull secrets in control planes. These require a
	// control plane context.
	PullSecret pullsecret.Cmd `cmd:"" help:"Manage package pull secrets."`

	// Commands for managing migrations from control planes. These require a
	// control plane context.
	Migration migration.Cmd `cmd:"" help:"Migrate control planes to Upbound Managed Control Planes."`

	// Commands for managing OIDC auth. These require a spaces control plane
	// context.
	OIDCAuth oidcauth.Cmd `cmd:"" help:"Create OIDC ProviderConfig in a Spaces control plane and Cloud Resources."`

	// Common Upbound API configuration
	Flags upbound.Flags `embed:""`
}

func extractSpaceFields(obj any) []string {
	ctp, ok := obj.(spacesv1beta1.ControlPlane)
	if !ok {
		return []string{"unknown", "unknown", "", "", "", "", ""}
	}

	v := ""
	if pv := ctp.Spec.Crossplane.Version; pv != nil {
		v = *pv
	}

	return []string{
		ctp.GetNamespace(),
		ctp.GetName(),
		v,
		string(ctp.GetCondition(xpcommonv1.TypeReady).Status),
		string(ctp.GetCondition(spacesv1beta1.ConditionTypeHealthy).Status),
		ctp.Status.Message,
		formatAge(ptr.To(time.Since(ctp.CreationTimestamp.Time))),
	}
}

func formatAge(age *time.Duration) string {
	if age == nil {
		return ""
	}

	return duration.HumanDuration(*age)
}

func tabularPrint(obj any, printer upterm.ObjectPrinter) error {
	spacefieldNames := []string{
		"GROUP",
		"NAME",
		"CROSSPLANE",
		"READY",
		"HEALTHY",
		"MESSAGE",
		"AGE",
	}
	return printer.Print(obj, spacefieldNames, extractSpaceFields)
}
