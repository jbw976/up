// Copyright 2025 Upbound Inc.
// All rights reserved

// Package token contains commands for working with robot tokens.
package token

import (
	"github.com/alecthomas/kong"

	"github.com/upbound/up-sdk-go/service/tokens"
	"github.com/upbound/up/internal/upbound"
)

const (
	errUserAccount      = "robots are not currently supported for user accounts"
	errMultipleRobotFmt = "found multiple robots with name %s in %s"
	errMultipleTokenFmt = "found multiple tokens with name %s for robot %s in %s"
	errFindRobotFmt     = "could not find robot %s in %s"
	errFindTokenFmt     = "could not find token %s for robot %s in %s"
)

// AfterApply constructs and binds a robots client to any subcommands
// that have Run() methods that receive it.
func (c *Cmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	cfg, err := upCtx.BuildSDKConfig()
	if err != nil {
		return err
	}
	kongCtx.Bind(tokens.NewClient(cfg))
	return nil
}

// Cmd contains commands for managing robot tokens.
type Cmd struct {
	Create createCmd `cmd:"" help:"Create a token for the robot."`
	Delete deleteCmd `cmd:"" help:"Delete a token for the robot."`
	List   listCmd   `cmd:"" help:"List the tokens for the robot."`
	Get    getCmd    `cmd:"" help:"Get a token for the robot."`
}
