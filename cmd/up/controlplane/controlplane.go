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
	"github.com/crossplane/crossplane-runtime/pkg/errors"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	"github.com/upbound/up/cmd/up/controlplane/connector"
	"github.com/upbound/up/cmd/up/controlplane/pkg"
	"github.com/upbound/up/cmd/up/controlplane/pullsecret"
	"github.com/upbound/up/cmd/up/controlplane/simulation"
	"github.com/upbound/up/internal/feature"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

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
	upCtx.SetupLogging()

	kongCtx.Bind(upCtx)

	// Pre-check the space context and control plane name
	_, ctp, isSpace := upCtx.GetCurrentSpaceContextScope()

	if isSpace { // Check if we are operating in a "space" context.
		// Define entities that require a control plane context.
		requiresControlPlane := map[string]bool{
			"function":      true,
			"provider":      true,
			"configuration": true,
			"pull-secret":   true,
		}

		// Get the selected parent's name.
		parentName := kongCtx.Selected().Parent.Name

		if requiresControlPlane[parentName] {
			// Ensure a control plane context is defined.
			if ctp.Name == "" {
				return errors.New("no control plane context is defined. Use 'up ctx' to set a control plane context")
			}
		} else {
			// Ensure we are not in a control plane context when one is not required.
			if ctp.Name != "" {
				return errors.New("cannot view control planes from inside a control plane context. Use 'up ctx ..' to go up to the group context")
			}
		}
	}
	cl, err := upCtx.BuildCurrentContextClient()
	if err != nil {
		return errors.Wrap(err, "unable to get kube client")
	}
	kongCtx.BindTo(cl, (*client.Client)(nil))

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
type Cmd struct {
	Create createCmd `cmd:"" help:"Create a control plane."`
	Delete deleteCmd `cmd:"" help:"Delete a control plane."`
	List   listCmd   `cmd:"" help:"List control planes for the organization."`
	Get    getCmd    `cmd:"" help:"Get a single control plane."`

	Connector connector.Cmd `cmd:"" help:"Connect an App Cluster to a control plane."`

	Simulation simulation.Cmd       `aliases:"sim" cmd:""                                                help:"Manage control plane simulations." maturity:"alpha"`
	Simulate   simulation.CreateCmd `cmd:""        help:"Alias for 'up controlplane simulation create'." maturity:"alpha"`

	Configuration pkg.Cmd `cmd:"" help:"Manage Configurations." set:"package_type=Configuration"`
	Provider      pkg.Cmd `cmd:"" help:"Manage Providers."      set:"package_type=Provider"`
	Function      pkg.Cmd `cmd:"" help:"Manage Functions."      set:"package_type=Function"`

	PullSecret pullsecret.Cmd `cmd:"" help:"Manage package pull secrets."`

	// Deprecated commands
	Connect    connectCmd    `cmd:"" help:"Deprecated: Connect kubectl to control plane."`
	Disconnect disconnectCmd `cmd:"" help:"Deprecated: Disconnect kubectl from control plane."`

	// Common Upbound API configuration
	Flags upbound.Flags `embed:""`
}

// Help returns the help text for the command.
// This method is required by the kong framework to provide command-line help functionality.
func (c *Cmd) Help() string {
	return `
Interact with control planes of the current profile. Use the "up ctx" command to
connect to a space or switch between contexts within a space.`
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
