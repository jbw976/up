// Copyright 2025 Upbound Inc.
// All rights reserved

package user

import (
	"context"

	"github.com/pterm/pterm"

	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

// inviteCmd sends out an invitation to a user to join an organization.
type inviteCmd struct {
	OrgName    string                                    `arg:""           help:"Name of the organization."            required:""`
	Email      string                                    `arg:""           help:"Email address of the user to invite." required:""`
	Permission organizations.OrganizationPermissionGroup `default:"member" enum:"member,owner"                         help:"Role of the user to invite (owner or member)." short:"p"`
}

// Run executes the invite command.
func (c *inviteCmd) Run(ctx context.Context, printer upterm.ObjectPrinter, p pterm.TextPrinter, oc *organizations.Client, upCtx *upbound.Context) error {
	orgID, err := oc.GetOrgID(ctx, c.OrgName)
	if err != nil {
		return err
	}

	if err = oc.CreateInvite(ctx, orgID, &organizations.OrganizationInviteCreateParameters{
		Email:      c.Email,
		Permission: c.Permission,
	}); err != nil {
		return err
	}

	p.Printfln("%s invited", c.Email)
	return nil
}
