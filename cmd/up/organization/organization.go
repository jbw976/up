// Copyright 2025 Upbound Inc.
// All rights reserved

// Package organization contains commands for working with Upbound
// Organizations.
package organization

import (
	"context"

	"github.com/alecthomas/kong"
	"github.com/posener/complete"

	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up/cmd/up/organization/user"
	"github.com/upbound/up/internal/upbound"
)

// AfterApply constructs and binds an organizations client to any subcommands
// that have Run() methods that receive it.
func (c *Cmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	cfg, err := upCtx.BuildSDKConfig()
	if err != nil {
		return err
	}
	kongCtx.Bind(upCtx)
	kongCtx.Bind(organizations.NewClient(cfg))
	return nil
}

// PredictOrgs predicts orgs for autocompletion.
func PredictOrgs() complete.Predictor {
	return complete.PredictFunc(func(_ complete.Args) (prediction []string) {
		upCtx, err := upbound.NewFromFlags(upbound.Flags{})
		if err != nil {
			return nil
		}
		upCtx.SetupLogging()

		cfg, err := upCtx.BuildSDKConfig()
		if err != nil {
			return nil
		}

		oc := organizations.NewClient(cfg)
		if oc == nil {
			return nil
		}

		orgs, err := oc.List(context.Background())
		if err != nil {
			return nil
		}

		if len(orgs) == 0 {
			return nil
		}

		data := make([]string, len(orgs))
		for i, o := range orgs {
			data[i] = o.Name
		}
		return data
	})
}

// Cmd contains commands for interacting with organizations.
type Cmd struct {
	upbound.RequiresContext

	Create createCmd `cmd:"" help:"Create an organization."`
	Delete deleteCmd `cmd:"" help:"Delete an organization."`
	List   listCmd   `cmd:"" help:"List organizations."`
	Get    getCmd    `cmd:"" help:"Get an organization."`

	User user.Cmd `cmd:"" help:"Manage organization users."`

	Token tokenCmd `cmd:"" help:"Generates an organization-scoped token to authenticate with a Cloud space."`
}
