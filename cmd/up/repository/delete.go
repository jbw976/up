// Copyright 2025 Upbound Inc.
// All rights reserved

package repository

import (
	"context"
	"fmt"

	"github.com/pterm/pterm"

	"github.com/upbound/up-sdk-go/service/repositories"
	"github.com/upbound/up/internal/input"
	"github.com/upbound/up/internal/upbound"
)

// BeforeApply sets default values for the delete command, before assignment and validation.
func (c *deleteCmd) BeforeApply() error {
	c.prompter = input.NewPrompter()
	return nil
}

// AfterApply accepts user input by default to confirm the delete operation.
func (c *deleteCmd) AfterApply(p pterm.TextPrinter, upCtx *upbound.Context) error {
	if c.Force {
		return nil
	}
	confirm, err := c.prompter.Prompt("Are you sure you want to delete this repository? [y/n]", false)
	if err != nil {
		return err
	}

	if input.InputYes(confirm) {
		p.Printfln("Deleting repository %s/%s. This cannot be undone.", upCtx.Organization, c.Name)
		return nil
	}

	return fmt.Errorf("operation canceled")
}

// deleteCmd deletes a repository on Upbound.
type deleteCmd struct {
	prompter input.Prompter

	Name string `arg:"" help:"Name of repository." predictor:"repos" required:""`

	Force bool `default:"false" help:"Force deletion of repository."`
}

// Run executes the delete command.
func (c *deleteCmd) Run(ctx context.Context, p pterm.TextPrinter, rc *repositories.Client, upCtx *upbound.Context) error {
	if err := rc.Delete(ctx, upCtx.Organization, c.Name); err != nil {
		return err
	}
	p.Printfln("%s/%s deleted", upCtx.Organization, c.Name)
	return nil
}
