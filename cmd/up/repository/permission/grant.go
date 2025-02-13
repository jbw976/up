// Copyright 2025 Upbound Inc.
// All rights reserved

package permission

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/pterm/pterm"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up-sdk-go/service/repositorypermission"
	"github.com/upbound/up/internal/upbound"
)

// grantCmd grant repositorypermission for an team on Upbound.
type grantCmd struct {
	TeamName       string `arg:"" help:"Name of team."                               required:""`
	RepositoryName string `arg:"" help:"Name of repository."                         required:""`
	Permission     string `arg:"" help:"Permission type (admin, read, write, view)." required:""`
}

// Validate validates the grantCmd struct.
func (c *grantCmd) Validate() error {
	switch repositorypermission.PermissionType(c.Permission) {
	case repositorypermission.PermissionAdmin, repositorypermission.PermissionRead, repositorypermission.PermissionWrite, repositorypermission.PermissionView:
		return nil
	default:
		return fmt.Errorf("invalid permission type %q: must be one of [admin, read, write, view]", c.Permission)
	}
}

// Run executes the create command.
func (c *grantCmd) Run(ctx context.Context, p pterm.TextPrinter, ac *accounts.Client, oc *organizations.Client, rpc *repositorypermission.Client, upCtx *upbound.Context) error {
	if err := c.Validate(); err != nil {
		return fmt.Errorf("permission validation failed for team %q in account %q: %w", c.TeamName, upCtx.Organization, err)
	}

	a, err := ac.Get(ctx, upCtx.Organization)
	if err != nil {
		return errors.Wrap(err, "cannot get accounts")
	}
	if a.Account.Type != accounts.AccountOrganization {
		return errors.New("user account is not an organization")
	}

	ts, err := oc.ListTeams(ctx, a.Organization.ID)
	if err != nil {
		return errors.Wrap(err, "cannot list teams")
	}

	// Find the team with the specified name
	var teamID uuid.UUID
	teamFound := false
	for _, t := range ts {
		if t.Name == c.TeamName {
			teamID = t.ID
			teamFound = true
			break
		}
	}
	if !teamFound {
		return fmt.Errorf("could not find team %q in account %q", c.TeamName, upCtx.Organization)
	}

	if err := rpc.Create(ctx, upCtx.Organization, teamID, repositorypermission.CreatePermission{
		Repository: c.RepositoryName,
		Permission: repositorypermission.RepositoryPermission{
			Permission: repositorypermission.PermissionType(c.Permission),
		},
	}); err != nil {
		return errors.Wrap(err, "cannot grant permission")
	}
	p.Printfln("Permission %q granted to team %q for repository %q in account %q", c.Permission, c.TeamName, c.RepositoryName, upCtx.Organization)
	return nil
}
