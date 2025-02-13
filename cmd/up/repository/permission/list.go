// Copyright 2025 Upbound Inc.
// All rights reserved

package permission

import (
	"context"
	"fmt"
	"time"

	"github.com/alecthomas/kong"
	"github.com/google/uuid"
	"github.com/pterm/pterm"
	"k8s.io/apimachinery/pkg/util/duration"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up-sdk-go/service/repositorypermission"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

// listCmd lists repository permissions for a team on Upbound.
type listCmd struct {
	TeamName string `arg:"" help:"Name of the team." required:""`
}

// AfterApply sets default values in command after assignment and validation.
func (c *listCmd) AfterApply(kongCtx *kong.Context) error {
	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))
	return nil
}

// Run executes the list command.
func (c *listCmd) Run(ctx context.Context, printer upterm.ObjectPrinter, p pterm.TextPrinter, ac *accounts.Client, oc *organizations.Client, rpc *repositorypermission.Client, upCtx *upbound.Context) error {
	// Get account details
	a, err := ac.Get(ctx, upCtx.Organization)
	if err != nil {
		return errors.Wrap(err, "cannot get accounts")
	}
	if a.Account.Type != accounts.AccountOrganization {
		return errors.New("user account is not an organization")
	}

	// Get the list of teams
	ts, err := oc.ListTeams(ctx, a.Organization.ID)
	if err != nil {
		return errors.Wrap(err, "cannot list teams")
	}

	// Create a map from team IDs to team names
	teamIDToName := make(map[uuid.UUID]string)
	for _, t := range ts {
		teamIDToName[t.ID] = t.Name
	}

	// Find the team ID for the specified team name
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
		return fmt.Errorf("could not find team %s in account %s", c.TeamName, upCtx.Organization)
	}

	// List repository permissions for the team
	resp, err := rpc.List(ctx, upCtx.Organization, teamID)
	if err != nil {
		return errors.Wrap(err, "cannot list permissions")
	}
	if len(resp.Permissions) == 0 {
		p.Printfln("No repository permissions found for team %s in account %s", c.TeamName, upCtx.Organization)
		return nil
	}

	fieldNames := []string{"TEAM", "REPOSITORY", "PERMISSION", "CREATED", "UPDATED"}
	return printer.Print(resp.Permissions, fieldNames, func(obj any) []string {
		return extractFields(obj, teamIDToName)
	})
}

// extractFields extracts the fields for printing.
func extractFields(obj any, teamIDToName map[uuid.UUID]string) []string {
	p := obj.(repositorypermission.Permission) //nolint:forcetypeassert // Type assertion will always be true because of what's passed to printer.Print above.

	updated := "n/a"
	if p.UpdatedAt != nil {
		updated = duration.HumanDuration(time.Since(*p.UpdatedAt))
	}

	teamName := teamIDToName[p.TeamID]

	return []string{teamName, p.RepositoryName, string(p.Privilege), p.CreatedAt.Format(time.RFC3339), updated}
}
