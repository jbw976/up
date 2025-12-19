// Copyright 2025 Upbound Inc.
// All rights reserved

package team

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/util/duration"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

//nolint:gochecknoglobals // Would make this a const if we could.
var fieldNames = []string{"NAME", "ID", "CREATED"}

// listCmd creates a team on Upbound.
type listCmd struct{}

// Run executes the list teams command.
func (c *listCmd) Run(ctx context.Context, printer upterm.Printer, ac *accounts.Client, oc *organizations.Client, upCtx *upbound.Context) error {
	a, err := ac.Get(ctx, upCtx.Organization)
	if err != nil {
		return err
	}

	if a.Account.Type != accounts.AccountOrganization {
		return errors.New(errUserAccount)
	}

	rs, err := oc.ListTeams(ctx, a.Organization.ID)
	if err != nil {
		return err
	}
	if len(rs) == 0 {
		printer.Printfln("No teams found in %s", upCtx.Organization)
		return nil
	}
	return printer.PrintObject(rs, fieldNames, extractFields)
}

func extractFields(obj any) []string {
	r := obj.(organizations.Team) //nolint:forcetypeassert // Assertion will always be true due to printer.Print call above.
	return []string{r.Name, r.ID.String(), duration.HumanDuration(time.Since(r.CreatedAt))}
}
