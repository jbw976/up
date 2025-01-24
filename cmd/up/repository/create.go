// Copyright 2025 Upbound Inc.
// All rights reserved

package repository

import (
	"context"

	"github.com/pterm/pterm"

	"github.com/upbound/up-sdk-go/service/repositories"
	"github.com/upbound/up/internal/upbound"
)

// createCmd creates a repository on Upbound.
type createCmd struct {
	Name    string `arg:""                                  help:"Name of repository." required:""`
	Private bool   `help:"Make the new repository private."`
}

// Run executes the create command.
func (c *createCmd) Run(ctx context.Context, p pterm.TextPrinter, rc *repositories.Client, upCtx *upbound.Context) error {
	visibility := repositories.WithPublic()
	if c.Private {
		visibility = repositories.WithPrivate()
	}
	if err := rc.CreateOrUpdateWithOptions(ctx, upCtx.Organization, c.Name, visibility); err != nil {
		return err
	}
	p.Printfln("%s/%s created", upCtx.Organization, c.Name)
	return nil
}
