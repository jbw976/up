// Copyright 2025 Upbound Inc.
// All rights reserved

package robot

import (
	"context"
	"strconv"

	"github.com/pterm/pterm"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/robots"
	"github.com/upbound/up/internal/upbound"
)

// createCmd creates a robot on Upbound.
type createCmd struct {
	Name string `arg:"" help:"Name of robot." required:""`

	// NOTE(hasheddan): a description is required by the API, but we default to
	// ' ' to avoid forcing the user to provide one.
	Description string `default:" " help:"Description of robot."`
}

// Run executes the create command.
func (c *createCmd) Run(ctx context.Context, p pterm.TextPrinter, ac *accounts.Client, rc *robots.Client, upCtx *upbound.Context) error {
	a, err := ac.Get(ctx, upCtx.Organization)
	if err != nil {
		return err
	}
	if a.Account.Type != accounts.AccountOrganization {
		return errors.New(errUserAccount)
	}
	if _, err := rc.Create(ctx, &robots.RobotCreateParameters{
		Attributes: robots.RobotAttributes{
			Name:        c.Name,
			Description: c.Description,
		},
		Relationships: robots.RobotRelationships{
			Owner: robots.RobotOwner{
				Data: robots.RobotOwnerData{
					Type: robots.RobotOwnerOrganization,
					ID:   strconv.FormatUint(uint64(a.Organization.ID), 10),
				},
			},
		},
	}); err != nil {
		return err
	}
	p.Printfln("%s/%s created", upCtx.Organization, c.Name)
	return nil
}
