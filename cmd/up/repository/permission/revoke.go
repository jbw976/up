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
	"github.com/upbound/up/internal/input"
	"github.com/upbound/up/internal/upbound"
)

// revokeCmd revoke the repository permission for a team on Upbound.
type revokeCmd struct {
	prompter input.Prompter

	TeamName       string `arg:""          help:"Name of team."                                                          required:""`
	RepositoryName string `arg:""          help:"Name of repository."                                                    required:""`
	Force          bool   `default:"false" help:"Force the revoke of the repository permission even if conflicts exist."`
}

// BeforeApply sets default values for the delete command, before assignment and validation.
func (c *revokeCmd) BeforeApply() error {
	c.prompter = input.NewPrompter()
	return nil
}

// AfterApply accepts user input by default to confirm the delete operation.
func (c *revokeCmd) AfterApply(p pterm.TextPrinter) error {
	if c.Force {
		return nil
	}

	confirm, err := c.prompter.Prompt(fmt.Sprintf("Are you sure you want to revoke the permission for team %q in repository %q? This cannot be undone [y/n]", c.TeamName, c.RepositoryName), false)
	if err != nil {
		return errors.Wrap(err, "error with revoke prompt")
	}

	if input.InputYes(confirm) {
		p.Printfln("Revoking repository permission for team %q in repository %q", c.TeamName, c.RepositoryName)
		return nil
	}

	return fmt.Errorf("operation canceled")
}

// Run executes the delete command.
func (c *revokeCmd) Run(ctx context.Context, p pterm.TextPrinter, ac *accounts.Client, oc *organizations.Client, rpc *repositorypermission.Client, upCtx *upbound.Context) error {
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

	if err := rpc.Delete(ctx, upCtx.Organization, teamID, repositorypermission.PermissionIdentifier{
		Repository: c.RepositoryName,
	}); err != nil {
		return errors.Wrap(err, "cannot revoke permission")
	}

	p.Printfln("Repository permission for team %q in repository %q revoked", c.TeamName, c.RepositoryName)
	return nil
}
