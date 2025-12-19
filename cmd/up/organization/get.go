// Copyright 2025 Upbound Inc.
// All rights reserved

package organization

import (
	"context"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up/internal/upterm"
)

// getCmd gets a single organization on Upbound.
type getCmd struct {
	Name string `arg:"" help:"Name of organization." predictor:"orgs" required:""`
}

// Run executes the get command.
func (c *getCmd) Run(printer upterm.ResultPrinter, oc *organizations.Client) error {
	// The get command accepts a name, but the get API call takes an ID
	// Therefore we get all orgs and find the one the user requested
	orgs, err := oc.List(context.Background())
	if err != nil {
		return err
	}
	for _, o := range orgs {
		if o.Name == c.Name {
			fieldNames := []string{"ID", "NAME", "ROLE"}
			return printer.PrintObject(o, fieldNames, extractFields)
		}
	}
	return errors.Errorf("no organization named %s", c.Name)
}
