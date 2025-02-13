// Copyright 2025 Upbound Inc.
// All rights reserved

package token

import (
	"context"
	"fmt"

	"github.com/alecthomas/kong"
	"github.com/google/uuid"
	"github.com/pterm/pterm"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/common"
	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up-sdk-go/service/robots"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

// AfterApply sets default values in command after assignment and validation.
func (c *getCmd) AfterApply(kongCtx *kong.Context) error {
	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))
	return nil
}

// getCmd get a robot token on Upbound.
type getCmd struct {
	RobotName string `arg:"" help:"Name of robot." required:""`
	TokenName string `arg:"" help:"Name of token." required:""`
}

// Run executes the get robot token command.
func (c *getCmd) Run(ctx context.Context, printer upterm.ObjectPrinter, ac *accounts.Client, oc *organizations.Client, rc *robots.Client, upCtx *upbound.Context) error {
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

	// We pick the first robot account with this name, though there
	// may be more than one. If a user wants to see all of the tokens
	// for robots with the same name, they can use the list commands
	var rid *uuid.UUID
	for _, r := range rs {
		if r.Name == c.RobotName {
			// Pin range variable so that we can take address.
			r := r
			rid = &r.ID
			break
		}
	}
	if rid == nil {
		return errors.Errorf(errFindRobotFmt, c.RobotName, upCtx.Organization)
	}

	ts, err := rc.ListTokens(ctx, *rid)
	if err != nil {
		return err
	}
	if len(ts.DataSet) == 0 {
		return errors.Errorf(errFindTokenFmt, c.TokenName, c.RobotName, upCtx.Organization)
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
		return errors.Errorf(errFindTokenFmt, c.TokenName, c.RobotName, upCtx.Organization)
	}
	return printer.Print(*theToken, fieldNames, extractFields)
}
