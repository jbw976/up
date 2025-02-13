// Copyright 2025 Upbound Inc.
// All rights reserved

package organization

import (
	"context"
	"strconv"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"

	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

// AfterApply sets default values in command after assignment and validation.
func (c *listCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))
	return nil
}

// listCmd lists organizations on Upbound.
type listCmd struct{}

var fieldNames = []string{"ID", "NAME", "ROLE"}

// Run executes the list command.
func (c *listCmd) Run(printer upterm.ObjectPrinter, p pterm.TextPrinter, oc *organizations.Client, upCtx *upbound.Context) error {
	orgs, err := oc.List(context.Background())
	if err != nil {
		return err
	}
	if len(orgs) == 0 {
		p.Printfln("No organizations found.")
		return nil
	}
	return printer.Print(orgs, fieldNames, extractFields)
}

func extractFields(obj any) []string {
	o := obj.(organizations.Organization)
	return []string{strconv.FormatUint(uint64(o.ID), 10), o.Name, string(o.Role)}
}
