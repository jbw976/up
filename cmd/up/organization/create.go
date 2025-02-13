// Copyright 2025 Upbound Inc.
// All rights reserved

package organization

import (
	"context"

	"github.com/pterm/pterm"

	"github.com/upbound/up-sdk-go/service/organizations"
)

// createCmd creates an organization on Upbound.
type createCmd struct {
	Name string `arg:"" help:"Name of organization." required:""`
}

// Run executes the create command.
func (c *createCmd) Run(ctx context.Context, p pterm.TextPrinter, oc *organizations.Client) error {
	if err := oc.Create(ctx, &organizations.OrganizationCreateParameters{
		Name: c.Name,
		// NOTE(hasheddan): we default display name to the same as name.
		DisplayName: c.Name,
	}); err != nil {
		return err
	}
	p.Printfln("%s created", c.Name)
	return nil
}
