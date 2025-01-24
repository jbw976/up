// Copyright 2025 Upbound Inc.
// All rights reserved

package team

import (
	"context"

	"github.com/google/uuid"
	"github.com/pterm/pterm"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up-sdk-go/service/robots"
	"github.com/upbound/up/internal/upbound"
)

// joinCmd adds a robot to team on Upbound.
type joinCmd struct {
	TeamName  string `arg:"" help:"Name of team."  required:""`
	RobotName string `arg:"" help:"Name of robot." required:""`
}

// Run executes the create command.
func (c *joinCmd) Run(ctx context.Context, p pterm.TextPrinter, ac *accounts.Client, oc *organizations.Client, rc *robots.Client, upCtx *upbound.Context) error {
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

	// Ensure exactly one robot with the specified name exists
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
		return errors.Errorf(errFindTeamFmt, c.TeamName, upCtx.Organization)
	}

	if err := rc.CreateTeamMembership(ctx, robotID, &robots.RobotTeamMembershipResourceIdentifier{
		Type: robots.RobotTeamMembershipTypeTeam,
		ID:   teamID.String(),
	}); err != nil {
		return err
	}
	p.Printfln("Adding robot %q to team %q", c.RobotName, c.TeamName)
	return nil
}
