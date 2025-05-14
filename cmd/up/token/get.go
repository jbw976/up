// Copyright 2025 Upbound Inc.
// All rights reserved

// Package token contains commands for working with personal user tokens.
package token

import (
	"context"
	"fmt"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/common"
	"github.com/upbound/up-sdk-go/service/users"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

// AfterApply sets default values in command after assignment and validation.
func (c *getCmd) AfterApply(kongCtx *kong.Context) error {
	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))
	return nil
}

// getCmd get a personal access token on Upbound.
type getCmd struct {
	TokenName string `arg:"" help:"Name of token." required:""`
}

// Run executes the get personal access token command.
func (c *getCmd) Run(ctx context.Context, printer upterm.ObjectPrinter, p pterm.TextPrinter, ac *accounts.Client, uc *users.Client, upCtx *upbound.Context) error {
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

	// We pick the first token with this name, though there may be more
	// than one. If a user wants to see all of the tokens with the same name
	// they can use the list command.
	var theToken *common.DataSet
	for _, t := range ts.DataSet {
		if fmt.Sprint(t.AttributeSet["name"]) == c.TokenName {
			// Pin range variable so that we can take address.
			t := t
			theToken = &t
			break
		}
	}
	if theToken == nil {
		return errors.Errorf(errFindTokenFmt, c.TokenName, upCtx.Profile.ID)
	}
	return printer.Print(*theToken, fieldNames, extractFields)
}
