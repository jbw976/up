// Copyright 2025 Upbound Inc.
// All rights reserved

// Package repository holds commands for working with xpkg repositories.
package repository

import (
	"context"

	"github.com/alecthomas/kong"
	"github.com/posener/complete"

	"github.com/upbound/up-sdk-go/service/common"
	"github.com/upbound/up-sdk-go/service/repositories"
	"github.com/upbound/up/cmd/up/repository/permission"
	"github.com/upbound/up/internal/upbound"
)

// AfterApply constructs and binds a repositories client to any subcommands
// that have Run() methods that receive it.
func (c *Cmd) AfterApply(kongCtx *kong.Context) error {
	upCtx, err := upbound.NewFromFlags(c.Flags)
	if err != nil {
		return err
	}
	upCtx.SetupLogging()

	cfg, err := upCtx.BuildSDKConfig()
	if err != nil {
		return err
	}
	kongCtx.Bind(upCtx)
	kongCtx.Bind(repositories.NewClient(cfg))
	return nil
}

// PredictRepos is the completion predictor for repositories.
func PredictRepos() complete.Predictor {
	return complete.PredictFunc(func(_ complete.Args) (prediction []string) {
		upCtx, err := upbound.NewFromFlags(upbound.Flags{})
		if err != nil {
			return nil
		}
		upCtx.SetupLogging()

		cfg, err := upCtx.BuildSDKConfig()
		if err != nil {
			return nil
		}

		rc := repositories.NewClient(cfg)
		if rc == nil {
			return nil
		}

		repos, err := rc.List(context.Background(), upCtx.Organization, common.WithSize(maxItems))
		if err != nil {
			return nil
		}

		if len(repos.Repositories) == 0 {
			return nil
		}

		data := make([]string, len(repos.Repositories))
		for i, o := range repos.Repositories {
			data[i] = o.Name
		}
		return data
	})
}

// Cmd contains commands for interacting with repositories.
type Cmd struct {
	Create     createCmd      `cmd:"" help:"Create a repository."`
	Delete     deleteCmd      `cmd:"" help:"Delete a repository."`
	List       listCmd        `cmd:"" help:"List repositories for the account."`
	Get        getCmd         `cmd:"" help:"Get a repository for the account."`
	Permission permission.Cmd `cmd:"" help:"Manage permissions of a repository for a team in the account."`

	// Common Upbound API configuration
	Flags upbound.Flags `embed:""`
}
