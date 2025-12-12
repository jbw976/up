// Copyright 2025 Upbound Inc.
// All rights reserved

package team

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

// getCmd gets a single team in an account on Upbound.
type getCmd struct {
	Name string `arg:"" help:"Name of team." predictor:"teams" required:""`
}

// Run executes the get team command.
func (c *getCmd) Run(ctx context.Context, printer upterm.ResultPrinter, ac *accounts.Client, oc *organizations.Client, upCtx *upbound.Context) error {
	a, err := ac.Get(ctx, upCtx.Organization)
	if err != nil {
		return err
	}

	if a.Account.Type != accounts.AccountOrganization {
		return errors.New(errUserAccount)
	}

	// The get command accepts a name, but the get API call takes an ID
	// Therefore we get all teams and find the one the user requested
	// The API doesn't guarantee uniqueness, but we just print the first
	// one we find. If a user wants to list all of them, they can use
	// the list command.
	rs, err := oc.ListTeams(ctx, a.Organization.ID)
	if err != nil {
		return err
	}

	for _, r := range rs {
		if r.Name == c.Name {
			return printer.PrintObject(r, fieldNames, extractFields)
		}
	}
	return fmt.Errorf("no team named %q", c.Name)
}
