// Copyright 2025 Upbound Inc.
// All rights reserved

package robot

import (
	"context"
	"time"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	"k8s.io/apimachinery/pkg/util/duration"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

//nolint:gochecknoglobals // Would make this a const if we could.
var fieldNames = []string{"NAME", "ID", "DESCRIPTION", "CREATED"}

// AfterApply sets default values in command after assignment and validation.
func (c *listCmd) AfterApply(kongCtx *kong.Context) error {
	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))
	return nil
}

// listCmd creates a robot on Upbound.
type listCmd struct{}

// Run executes the list robots command.
func (c *listCmd) Run(ctx context.Context, printer upterm.ObjectPrinter, p pterm.TextPrinter, ac *accounts.Client, oc *organizations.Client, upCtx *upbound.Context) error {
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
		p.Printfln("No robots found in %s", upCtx.Organization)
		return nil
	}
	return printer.Print(rs, fieldNames, extractFields)
}

func extractFields(obj any) []string {
	r := obj.(organizations.Robot) //nolint:forcetypeassert // Type assertion will always be true because of what's passed to printer.Print above.
	return []string{r.Name, r.ID.String(), r.Description, duration.HumanDuration(time.Since(r.CreatedAt))}
}
