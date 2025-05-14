// Copyright 2025 Upbound Inc.
// All rights reserved

// Package token contains commands for working with personal user tokens.
package token

import (
	"github.com/alecthomas/kong"

	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/robots"
	"github.com/upbound/up-sdk-go/service/tokens"
	"github.com/upbound/up-sdk-go/service/users"
	"github.com/upbound/up/internal/upbound"
)

const (
	errRobot            = "robots cannot create personal access tokens"
	errFindTokenFmt     = "could not find personal access token %s for %s"
	errMultipleTokenFmt = "found multiple tokens with name %s for current user %s"
)

// AfterApply constructs and binds a needed clients to any subcommands
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
	kongCtx.Bind(accounts.NewClient(cfg))
	kongCtx.Bind(tokens.NewClient(cfg))
	kongCtx.Bind(robots.NewClient(cfg))
	kongCtx.Bind(users.NewClient(cfg))
	return nil
}

// Cmd contains commands for managing robot tokens.
type Cmd struct {
	Create createCmd `cmd:"" help:"Create a personal access token for the current user."`
	List   listCmd   `cmd:"" help:"Get all personal access tokens for the current user."`
	Get    getCmd    `cmd:"" help:"Get a personal access token for the current user."`
	Delete deleteCmd `cmd:"" help:"Delete a personal access token for the current user."`

	// Common Upbound API configuration
	Flags upbound.Flags `embed:""`
}
