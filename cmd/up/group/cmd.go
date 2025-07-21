// Copyright 2025 Upbound Inc.
// All rights reserved

// Package group contains commands for working with groups in spaces.
package group

import (
	"strconv"

	"github.com/alecthomas/kong"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	"github.com/upbound/up/internal/feature"
	"github.com/upbound/up/internal/style"
	"github.com/upbound/up/internal/upbound"
)

var fieldNames = []string{"NAME", "PROTECTED"} //nolint:gochecknoglobals // Const used by list and get.

func init() {
	runtime.Must(spacesv1beta1.AddToScheme(scheme.Scheme))
}

// BeforeReset is the first hook to run.
func (c *Cmd) BeforeReset(p *kong.Path, maturity feature.Maturity) error {
	return feature.HideMaturity(p, maturity)
}

// AfterApply constructs and binds an Upbound context to any subcommands
// that have Run() methods that receive it.
func (c *Cmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	// we can't use groups from inside a control plane
	if _, ctp, inSpace := upCtx.GetCurrentSpaceContextScope(); !inSpace {
		return errors.New("your kubeconfig must be pointing at a space context")
	} else if ctp.Name != "" {
		return errors.New("cannot view groups from inside a control plane context. Use 'up ctx ..' to go up to the group context")
	}

	cl, err := upCtx.BuildCurrentContextClient()
	if err != nil {
		return errors.Wrap(err, "unable to get kube client")
	}

	kongCtx.BindTo(cl, (*client.Client)(nil))

	return nil
}

// Cmd contains commands for interacting with groups.
type Cmd struct {
	upbound.RequiresContext

	Create createCmd `cmd:"" help:"Create a group."`
	Delete deleteCmd `cmd:"" help:"Delete a group."`
	List   listCmd   `cmd:"" help:"List groups in the space."`
	Get    getCmd    `cmd:"" help:"Get a group."`
}

// Help returns help text.
func (c *Cmd) Help() string {
	return style.RenderHelp(`
The <group> command interacts with groups within the current space. Both Upbound profiles and
local Spaces are supported. Use the "profile" management command to switch
between different Upbound profiles or to connect to a local Space.

## Usage Examples:

    up group list
        List all groups in the current space.
        Shows group names and protection status.

    up group create <my-group>
        Create a new group named "my-group".
        Groups organize control planes within a space.

    up group get <my-group>
        Get details about a specific group.
        Shows group configuration and metadata.

    up group delete <my-group>
        Delete a group.
        Cannot delete protected groups.
`)
}

func extractGroupFields(obj any) []string {
	resp, ok := obj.(corev1.Namespace)
	if !ok {
		return []string{"unknown", "unknown"}
	}

	protected := false
	if av, ok := resp.ObjectMeta.Labels[spacesv1beta1.ControlPlaneGroupProtectionKey]; ok {
		if val, err := strconv.ParseBool(av); err == nil {
			protected = val
		}
	}

	return []string{
		resp.GetObjectMeta().GetName(),
		strconv.FormatBool(protected),
	}
}
