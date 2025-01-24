// Copyright 2025 Upbound Inc.
// All rights reserved

package team

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/pterm/pterm"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up-sdk-go/service/teams"
	"github.com/upbound/up/internal/input"
	"github.com/upbound/up/internal/upbound"
)

const (
	errMultipleTeamFmt = "found multiple teams with name %s in %s"
	errFindTeamFmt     = "could not find team %s in %s"
)

// BeforeApply sets default values for the delete command, before assignment and validation.
func (c *deleteCmd) BeforeApply() error {
	c.prompter = input.NewPrompter()
	return nil
}

// AfterApply accepts user input by default to confirm the delete operation.
func (c *deleteCmd) AfterApply(p pterm.TextPrinter, upCtx *upbound.Context) error {
	if c.Force {
		return nil
	}

	confirm, err := c.prompter.Prompt("Are you sure you want to delete this team? This cannot be undone. [y/n]", false)
	if err != nil {
		return err
	}

	if input.InputYes(confirm) {
		p.Printfln("Deleting team %s/%s. ", upCtx.Organization, c.Name)
		return nil
	}

	return fmt.Errorf("operation canceled")
}

// deleteCmd deletes a team on Upbound.
type deleteCmd struct {
	prompter input.Prompter

	Name string `arg:"" help:"Name of team." predictor:"teams" required:""`

	Force bool `default:"false" help:"Force delete team even if conflicts exist."`
}

// Run executes the delete command.
func (c *deleteCmd) Run(ctx context.Context, p pterm.TextPrinter, ac *accounts.Client, oc *organizations.Client, tc *teams.Client, upCtx *upbound.Context) error {
	a, err := ac.Get(ctx, upCtx.Organization)
	if err != nil {
		return err
	}

	if a.Account.Type != accounts.AccountOrganization {
		return errors.New(errUserAccount)
	}

	rs, err := oc.ListTeams(ctx, a.Organization.ID)
	if err != nil {
		return err
	}
	if len(rs) == 0 {
		return errors.Errorf(errFindTeamFmt, c.Name, upCtx.Organization)
	}

	var id *uuid.UUID
	for _, r := range rs {
		if r.Name == c.Name {
			if id != nil && !c.Force {
				return errors.Errorf(errMultipleTeamFmt, c.Name, upCtx.Organization)
			}
			r := r
			id = &r.ID
		}
	}

	if id == nil {
		return errors.Errorf(errFindTeamFmt, c.Name, upCtx.Organization)
	}

	if err := tc.Delete(ctx, *id); err != nil {
		return err
	}
	p.Printfln("%s/%s deleted", upCtx.Organization, c.Name)
	return nil
}
