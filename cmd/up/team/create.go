// Copyright 2025 Upbound Inc.
// All rights reserved

package team

import (
	"context"

	"github.com/pterm/pterm"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/teams"
	"github.com/upbound/up/internal/upbound"
)

// createCmd creates a team on Upbound.
type createCmd struct {
	Name string `arg:"" help:"Name of Team." required:""`
}

// Run executes the create command.
func (c *createCmd) Run(ctx context.Context, p pterm.TextPrinter, ac *accounts.Client, tc *teams.Client, upCtx *upbound.Context) error {
	a, err := ac.Get(ctx, upCtx.Organization)
	if err != nil {
		return err
	}

	if a.Account.Type != accounts.AccountOrganization {
		return errors.New(errUserAccount)
	}

	if _, err := tc.Create(ctx, &teams.TeamCreateParameters{
		Name:           c.Name,
		OrganizationID: a.Organization.ID,
	},
	); err != nil {
		return err
	}
	p.Printfln("%s/%s created", upCtx.Organization, c.Name)
	return nil
}
