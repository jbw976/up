// Copyright 2025 Upbound Inc.
// All rights reserved

package robot

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

const (
	errMultipleRobotFmt = "found multiple robots with name %s in %s"
	errFindRobotFmt     = "could not find robot %s in %s"
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

	confirm, err := c.prompter.Prompt("Are you sure you want to delete this robot? [y/n]", false)
	if err != nil {
		return err
	}

	if input.InputYes(confirm) {
		p.Printfln("Deleting robot %s/%s. This cannot be undone.", upCtx.Organization, c.Name)
		return nil
	}

	return fmt.Errorf("operation canceled")
}

// deleteCmd deletes a robot on Upbound.
type deleteCmd struct {
	prompter input.Prompter

	Name string `arg:"" help:"Name of robot." predictor:"robots" required:""`

	Force bool `default:"false" help:"Force delete robot even if conflicts exist."`
}

// Run executes the delete command.
func (c *deleteCmd) Run(ctx context.Context, p pterm.TextPrinter, ac *accounts.Client, oc *organizations.Client, rc *robots.Client, upCtx *upbound.Context) error {
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
		return errors.Errorf(errFindRobotFmt, c.Name, upCtx.Organization)
	}
	// TODO(hasheddan): because this API does not guarantee name uniqueness, we
	// must guarantee that exactly one robot exists in the specified account
	// with the provided name. Logic should be simplified when the API is
	// updated.
	var id *uuid.UUID
	for _, r := range rs {
		if r.Name == c.Name {
			if id != nil && !c.Force {
				return errors.Errorf(errMultipleRobotFmt, c.Name, upCtx.Organization)
			}
			// Pin range variable so that we can take address.
			r := r
			id = &r.ID
		}
	}

	if id == nil {
		return errors.Errorf(errFindRobotFmt, c.Name, upCtx.Organization)
	}

	if err := rc.Delete(ctx, *id); err != nil {
		return err
	}
	p.Printfln("%s/%s deleted", upCtx.Organization, c.Name)
	return nil
}
