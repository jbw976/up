// Copyright 2025 Upbound Inc.
// All rights reserved

// Package token contains commands for working with personal user tokens.
package token

import (
	"context"
	"fmt"
	"time"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	"k8s.io/apimachinery/pkg/util/duration"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/common"
	"github.com/upbound/up-sdk-go/service/users"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

//nolint:gochecknoglobals // Would make this a const if we could.
var fieldNames = []string{"NAME", "ID", "CREATED"}

// AfterApply sets default values in command after assignment and validation.
func (c *listCmd) AfterApply(kongCtx *kong.Context) error {
	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))
	return nil
}

// listCmd list all tokens from current user.
type listCmd struct{}

// Run executes the list personal access tokens command.
func (c *listCmd) Run(ctx context.Context, printer upterm.ObjectPrinter, p pterm.TextPrinter, ac *accounts.Client, uc *users.Client, upCtx *upbound.Context) error {
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
	return printer.Print(ts.DataSet, fieldNames, extractFields)
}

func extractFields(obj any) []string {
	t := obj.(common.DataSet) //nolint:forcetypeassert // Type assertion will always be true because of what's passed to printer.Print above.

	n := fmt.Sprint(t.AttributeSet["name"])
	c := "n/a"
	if ca, ok := t.Meta["createdAt"]; ok {
		if ct, err := time.Parse(time.RFC3339, fmt.Sprint(ca)); err == nil {
			c = duration.HumanDuration(time.Since(ct))
		}
	}
	return []string{n, t.ID.String(), c}
}
