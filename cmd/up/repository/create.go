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
	Name string `arg:"" help:"Name of repository." required:""`

	Private bool `help:"Make the new repository private."`
	Publish bool `help:"Enable Upbound Marketplace listing page for the new repository."`
}

// Run executes the create command.
func (c *createCmd) Run(ctx context.Context, p pterm.TextPrinter, rc *repositories.Client, upCtx *upbound.Context) error {
	// Defaults are public visibility and no indexing (publishing).
	// The server does handle unset fields, but since this is a PUT endpoint we'll explicitly set every field in the request.
	visibility := repositories.WithPublic()
	publishPolicy := repositories.WithDraft()
	if c.Private {
		visibility = repositories.WithPrivate()
	}
	if c.Publish {
		publishPolicy = repositories.WithPublish()
	}
	if err := rc.CreateOrUpdateWithOptions(ctx, upCtx.Organization, c.Name, visibility, publishPolicy); err != nil {
		return err
	}
	p.Printfln("%s/%s created", upCtx.Organization, c.Name)
	return nil
}
