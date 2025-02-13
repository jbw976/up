// Copyright 2025 Upbound Inc.
// All rights reserved

package repository

import (
	"context"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"

	repos "github.com/upbound/up-sdk-go/service/repositories"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

// AfterApply sets default values in command after assignment and validation.
func (c *getCmd) AfterApply(kongCtx *kong.Context) error {
	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))
	return nil
}

// getCmd gets a single repo.
type getCmd struct {
	Name string `arg:"" help:"Name of repo." predictor:"repos" required:""`
}

// Run executes the get command.
func (c *getCmd) Run(ctx context.Context, printer upterm.ObjectPrinter, rc *repos.Client, upCtx *upbound.Context) error {
	repo, err := rc.Get(ctx, upCtx.Organization, c.Name)
	if err != nil {
		return err
	}

	// We convert to a list so we can match the output of the list command
	repoList := repos.RepositoryListResponse{
		Repositories: []repos.Repository{repo.Repository},
	}
	return printer.Print(repoList.Repositories, fieldNames, extractFields)
}
