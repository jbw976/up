// Copyright 2025 Upbound Inc.
// All rights reserved

// Package token contains commands for working with personal user tokens.
package token

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/pterm/pterm"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/tokens"
	"github.com/upbound/up-sdk-go/service/users"
	"github.com/upbound/up/internal/input"
	"github.com/upbound/up/internal/upbound"
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

	confirm, err := c.prompter.Prompt("Are you sure you want to delete this personal access token? [y/n]", false)
	if err != nil {
		return err
	}

	if input.InputYes(confirm) {
		p.Printfln("Deleting personal access token %s/%s. This cannot be undone.", upCtx.Profile.ID, c.TokenName)
		return nil
	}

	return fmt.Errorf("operation canceled")
}

// deleteCmd deletes a personal access token on Upbound.
type deleteCmd struct {
	prompter input.Prompter

	TokenName string `arg:"" help:"Name of token." required:""`

	Force bool `default:"false" help:"Force delete token even if conflicts exist."`
}

// Run executes the delete command.
func (c *deleteCmd) Run(ctx context.Context, p pterm.TextPrinter, ac *accounts.Client, tc *tokens.Client, uc *users.Client, upCtx *upbound.Context) error {
	a, err := ac.Get(ctx, upCtx.Organization)
	if err != nil {
		return err
	}
	if a.Account.Type != accounts.AccountOrganization {
		return errors.New(errRobot)
	}

	// get the userID
	u, err := ac.Get(ctx, upCtx.Profile.ID)
	if err != nil {
		return err
	}

	ts, err := uc.ListTokens(ctx, u.Organization.CreatorID)
	if err != nil {
		return err
	}
	if len(ts.DataSet) == 0 {
		p.Printfln("No personal access tokens found for user %s in %s", upCtx.Profile.ID, upCtx.Organization)
		return nil
	}

	// TODO(hasheddan): because this API does not guarantee name uniqueness, we
	// must guarantee that exactly one token exists for the specified robot in
	// the specified account with the provided name. Logic should be simplified
	// when the API is updated.
	var tid *uuid.UUID
	for _, t := range ts.DataSet {
		if fmt.Sprint(t.AttributeSet["name"]) == c.TokenName {
			if tid != nil && !c.Force {
				return errors.Errorf(errMultipleTokenFmt, c.TokenName, upCtx.Profile.ID)
			}
			// Pin range variable so that we can take address.
			t := t
			tid = &t.ID
		}
	}
	if tid == nil {
		return errors.Errorf(errFindTokenFmt, c.TokenName, upCtx.Profile.ID)
	}

	if err := tc.Delete(ctx, *tid); err != nil {
		return err
	}
	p.Printfln("%s/%s deleted", upCtx.Profile.ID, c.TokenName)
	return nil
}
