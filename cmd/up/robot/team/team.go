// Copyright 2025 Upbound Inc.
// All rights reserved

// Package team contains commands for manipulating a robot token's team
// assignments.
package team

import (
	"github.com/alecthomas/kong"

	"github.com/upbound/up-sdk-go/service/robots"
	"github.com/upbound/up-sdk-go/service/teams"
	"github.com/upbound/up/internal/upbound"
)

const (
	errUserAccount      = "robots are not currently supported for user accounts"
	errMultipleRobotFmt = "found multiple robots with name %s in %s"
	errFindRobotFmt     = "could not find robot %s in %s"
	errFindTeamFmt      = "could not find team %s in %s"
)

// AfterApply constructs and binds a robots client to any subcommands
// that have Run() methods that receive it.
func (c *Cmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	cfg, err := upCtx.BuildSDKConfig()
	if err != nil {
		return err
	}
	kongCtx.Bind(robots.NewClient(cfg))
	kongCtx.Bind(teams.NewClient(cfg))
	return nil
}

// Cmd contains commands for managing robot teams.
type Cmd struct {
	Join  joinCmd  `cmd:"" help:"Add the robot to a team."`
	Leave leaveCmd `cmd:"" help:"Remove the robot from a team."`
	List  listCmd  `cmd:"" help:"List all teams the robot is a member of."`
}
