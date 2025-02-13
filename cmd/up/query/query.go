// Copyright 2025 Upbound Inc.
// All rights reserved

// Please note: As of March 2023, the `upbound` commands have been disabled.
// We're keeping the code here for now, so they're easily resurrected.
// The upbound commands were meant to support the Upbound self-hosted option.

// Package query contains commands for querying control plane resources via the
// Query API.
package query

import (
	"fmt"

	"github.com/alecthomas/kong"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/cmd/up/query/resource"
	"github.com/upbound/up/internal/feature"
	"github.com/upbound/up/internal/upbound"
)

// QueryCmd is the `up alpha query` command.
//
//nolint:revive // QueryCmd stutters, but differentiates this command from GetCmd.
type QueryCmd struct {
	cmd

	// flags about the scope
	Namespace    string `env:"UPBOUND_NAMESPACE"     help:"Namespace name for resources to query. By default, it's all namespaces if not on a control plane profile   the profiles current namespace or \"default\"." name:"namespace"    short:"n"`
	Group        string `env:"UPBOUND_GROUP"         help:"Control plane group. By default, it's the kubeconfig's current namespace or \"default\"."                                                                  name:"group"        short:"g"`
	ControlPlane string `env:"UPBOUND_CONTROLPLANE"  help:"Control plane name. Defaults to the current kubeconfig context if it points to a control plane."                                                           name:"controlplane" short:"c"`
	AllGroups    bool   `help:"Query in all groups." name:"all-groups"                                                                                                                                                short:"A"`
}

// BeforeReset is the first hook to run.
func (c *QueryCmd) BeforeReset(p *kong.Path, maturity feature.Maturity) error {
	return feature.HideMaturity(p, maturity)
}

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *QueryCmd) AfterApply(kongCtx *kong.Context) error { //nolint:gocyclo // pure plumbing. Doesn't get better by splitting.
	upCtx, err := upbound.NewFromFlags(c.Flags, upbound.AllowMissingProfile())
	if err != nil {
		return err
	}
	upCtx.SetupLogging()

	kongCtx.Bind(upCtx)

	kubeconfig, err := upCtx.GetKubeconfig()
	if err != nil {
		return err
	}

	// where are we?
	base, ctp, isSpace := upCtx.GetCurrentSpaceContextScope()
	if !isSpace {
		return errors.New("not connected to a Space; use 'up ctx' to switch to a Space or control plane context")
	}
	if ctp.Namespace != "" && ctp.Name != "" {
		// on a controlplane
		if c.AllGroups {
			return errors.Errorf("cannot use --all-groups in a control plane context; use `up ctx ..' to switch to the Space")
		}
		if c.Group != "" {
			return errors.Errorf("cannot use --group in a control plane context; use `up ctx ..' to switch to the Space")
		}
		c.Group = ctp.Namespace
		c.ControlPlane = ctp.Name

		// move from ctp URL to Spaces API in order to send Query API requests
		kubeconfig.Host = base
		kubeconfig.APIPath = ""
	} else if c.Group == "" && !c.AllGroups {
		// on the Spaces API
		if ctp.Namespace != "" {
			c.Group = ctp.Namespace
		}
	}

	kongCtx.Bind(kubeconfig)

	// create query template, kind depending on the scope
	var query resource.QueryObject
	switch {
	case c.Group != "" && c.ControlPlane != "":
		query = &resource.Query{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: c.Group,
				Name:      c.ControlPlane,
			},
		}
	case c.Group != "":
		query = &resource.GroupQuery{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: c.Group,
			},
		}
	default:
		query = &resource.SpaceQuery{}
	}
	kongCtx.BindTo(query, (*resource.QueryObject)(nil))

	// namespace in the control plane, logic is easy here
	c.namespace = c.Namespace

	// what to print if there is no resource found
	kongCtx.BindTo(NotFoundFunc(func() error {
		if c.namespace != "" {
			switch {
			case c.Group == "":
				_, err = fmt.Fprintf(kongCtx.Stderr, "No resources found in %q namespace in any control plane.\n", c.namespace)
			case c.ControlPlane == "":
				_, err = fmt.Fprintf(kongCtx.Stderr, "No resources found in %s namespace in control plane group %q.\n", c.namespace, c.Group)
			default:
				_, err = fmt.Fprintf(kongCtx.Stderr, "No resources found in %q namespace in control plane %s/%s.\n", c.namespace, c.Group, c.ControlPlane)
			}
		} else {
			switch {
			case c.Group == "":
				_, err = fmt.Fprintln(kongCtx.Stderr, "No resources found in any control plane.")
			case c.ControlPlane == "":
				_, err = fmt.Fprintf(kongCtx.Stderr, "No resources found in control plane group %q.\n", c.Group)
			default:
				_, err = fmt.Fprintf(kongCtx.Stderr, "No resources found in control plane %s/%s.\n", c.Group, c.ControlPlane)
			}
		}
		return err
	}), (*NotFound)(nil))

	return c.afterApply()
}

// Help returns help for the query command.
func (c *QueryCmd) Help() string {
	s, err := help("up alpha query")
	if err != nil {
		return err.Error()
	}
	return s
}
