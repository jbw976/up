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
	"github.com/upbound/up-sdk-go/service/robots"
	"github.com/upbound/up/internal/input"
	"github.com/upbound/up/internal/upbound"
)

// BeforeApply sets default values for the delete command, before assignment and validation.
func (c *leaveCmd) BeforeApply() error {
	c.prompter = input.NewPrompter()
	return nil
}

// AfterApply accepts user input by default to confirm the delete operation.
func (c *leaveCmd) AfterApply(p pterm.TextPrinter) error {
	if c.Force {
		return nil
	}

	confirm, err := c.prompter.Prompt("Are you sure you want to remove this robot from the team? This cannot be undone [y/n]", false)
	if err != nil {
		return err
	}

	if input.InputYes(confirm) {
		p.Printfln("Removing robot %q from team %q", c.RobotName, c.TeamName)
		return nil
	}

	return fmt.Errorf("operation canceled")
}

// leaveCmd removes the robot from a team on Upbound.
type leaveCmd struct {
	prompter input.Prompter

	TeamName  string `arg:"" help:"Name of team."  required:""`
	RobotName string `arg:"" help:"Name of robot." required:""`

	Force bool `default:"false" help:"Force the removal of a robot from a team even if conflicts exist."`
}

// Run executes the delete command.
func (c *leaveCmd) Run(ctx context.Context, p pterm.TextPrinter, ac *accounts.Client, oc *organizations.Client, rc *robots.Client, upCtx *upbound.Context) error {
	a, err := ac.Get(ctx, upCtx.Organization)
	if err != nil {
		return err
	}
	if a.Account.Type != accounts.AccountOrganization {
		return errors.New(errUserAccount)
	}

	rs, err := oc.ListRobots(ctx, a.Organization.ID)
	if err != nil {
		return err
	}
	if len(rs) == 0 {
		return errors.Errorf(errFindRobotFmt, c.RobotName, upCtx.Organization)
	}

	var robotID uuid.UUID
	robotCount := 0
	for _, r := range rs {
		if r.Name == c.RobotName {
			robotID = r.ID
			robotCount++
		}
	}
	if robotCount == 0 {
		return errors.Errorf(errFindRobotFmt, c.RobotName, upCtx.Organization)
	}
	if robotCount > 1 {
		return errors.Errorf(errMultipleRobotFmt, c.RobotName, upCtx.Organization)
	}

	ts, err := oc.ListTeams(ctx, a.Organization.ID)
	if err != nil {
		return err
	}

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
		return errors.Errorf(errFindTeamFmt, c.TeamName, upCtx.Organization)
	}

	if err := rc.DeleteTeamMembership(ctx, robotID, &robots.RobotTeamMembershipResourceIdentifier{
		Type: robots.RobotTeamMembershipTypeTeam,
		ID:   teamID.String(),
	}); err != nil {
		return err
	}

	p.Printfln("Removed robot %q from team %q", c.RobotName, c.TeamName)
	return nil
}
