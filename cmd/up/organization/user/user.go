// Copyright 2025 Upbound Inc.
// All rights reserved

// Package user contains commands for working with users in an organization.
package user

import (
	"github.com/alecthomas/kong"

	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up/internal/upbound"
)

// AfterApply constructs and binds a robots client to any subcommands
// that have Run() methods that receive it.
func (c *Cmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	cfg, err := upCtx.BuildSDKConfig()
	if err != nil {
		return err
	}
	kongCtx.Bind(organizations.NewClient(cfg))
	return nil
}

// Cmd contains commands for managing organization users.
type Cmd struct {
	List   listCmd   `cmd:"" help:"List users of an organization."`
	Invite inviteCmd `cmd:"" help:"Invite a user to the organization."`
	Remove removeCmd `cmd:"" help:"Remove a member from the organization."`
}
