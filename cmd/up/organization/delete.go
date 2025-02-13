// Copyright 2025 Upbound Inc.
// All rights reserved

package organization

import (
	"context"
	"fmt"

	"github.com/pterm/pterm"

	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up/internal/input"
)

// BeforeApply sets default values for the delete command, before assignment and validation.
func (c *deleteCmd) BeforeApply() error {
	c.prompter = input.NewPrompter()
	return nil
}

// AfterApply accepts user input by default to confirm the delete operation.
func (c *deleteCmd) AfterApply(p pterm.TextPrinter) error {
	if c.Force {
		return nil
	}

	confirm, err := c.prompter.Prompt("Are you sure you want to delete this organization? [y/n]", false)
	if err != nil {
		return err
	}

	if input.InputYes(confirm) {
		p.Printfln("Deleting organization %s. This cannot be undone.", c.Name)
		return nil
	}

	return fmt.Errorf("operation canceled")
}

// deleteCmd deletes an organization on Upbound.
type deleteCmd struct {
	prompter input.Prompter

	Name string `arg:"" help:"Name of organization." predictor:"orgs" required:""`

	Force bool `default:"false" help:"Force deletion of the organization."`
}

// Run executes the delete command.
func (c *deleteCmd) Run(ctx context.Context, p pterm.TextPrinter, oc *organizations.Client) error {
	id, err := oc.GetOrgID(ctx, c.Name)
	if err != nil {
		return err
	}
	if err := oc.Delete(ctx, id); err != nil {
		return err
	}
	p.Printfln("%s deleted", c.Name)
	return nil
}
