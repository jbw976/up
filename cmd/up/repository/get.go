// Copyright 2025 Upbound Inc.
// All rights reserved

package repository

import (
	"context"

	repos "github.com/upbound/up-sdk-go/service/repositories"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

// getCmd gets a single repo.
type getCmd struct {
	Name string `arg:"" help:"Name of repo." predictor:"repos" required:""`
}

// Run executes the get command.
func (c *getCmd) Run(ctx context.Context, printer upterm.ResultPrinter, rc *repos.Client, upCtx *upbound.Context) error {
	repo, err := rc.Get(ctx, upCtx.Organization, c.Name)
	if err != nil {
		return err
	}

	// We convert to a list so we can match the output of the list command
	repoList := repos.RepositoryListResponse{
		Repositories: []repos.Repository{repo.Repository},
	}
	return printer.PrintObject(repoList.Repositories, fieldNames, extractFields)
}
