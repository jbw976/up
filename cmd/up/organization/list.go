// Copyright 2025 Upbound Inc.
// All rights reserved

package organization

import (
	"context"
	"strconv"

	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up/internal/upterm"
)

// listCmd lists organizations on Upbound.
type listCmd struct{}

// Run executes the list command.
func (c *listCmd) Run(ctx context.Context, printer upterm.Printer, oc *organizations.Client) error {
	orgs, err := oc.List(ctx)
	if err != nil {
		return err
	}
	if len(orgs) == 0 {
		printer.Printfln("No organizations found.")
		return nil
	}
	fieldNames := []string{"ID", "NAME", "ROLE"}
	return printer.PrintObject(orgs, fieldNames, extractFields)
}

func extractFields(obj any) []string {
	o, _ := obj.(organizations.Organization)
	return []string{strconv.FormatUint(uint64(o.ID), 10), o.Name, string(o.Role)}
}
