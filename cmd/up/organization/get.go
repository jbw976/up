// Copyright 2025 Upbound Inc.
// All rights reserved

package organization

import (
	"context"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

// AfterApply sets default values in command after assignment and validation.
func (c *getCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))
	return nil
}

// getCmd gets a single organization on Upbound.
type getCmd struct {
	Name string `arg:"" help:"Name of organization." predictor:"orgs" required:""`
}

// Run executes the get command.
func (c *getCmd) Run(printer upterm.ObjectPrinter, oc *organizations.Client, upCtx *upbound.Context) error {
	// The get command accepts a name, but the get API call takes an ID
	// Therefore we get all orgs and find the one the user requested
	orgs, err := oc.List(context.Background())
	if err != nil {
		return err
	}
	for _, o := range orgs {
		if o.Name == c.Name {
			return printer.Print(o, fieldNames, extractFields)
		}
	}
	return errors.Errorf("no organization named %s", c.Name)
}
