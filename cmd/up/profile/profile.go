// Copyright 2025 Upbound Inc.
// All rights reserved

// Package profile contains commands for working with configuration profiles.
package profile

import (
	"github.com/alecthomas/kong"
	"github.com/posener/complete"

	"github.com/upbound/up/internal/upbound"
)

// Cmd contains commands for Upbound Profiles.
type Cmd struct {
	upbound.RequiresContextAllowMissingProfile

	Current currentCmd `cmd:"" help:"Get the current active Upbound profile."`
	List    listCmd    `cmd:"" help:"List all configured Upbound profiles."`
	Use     useCmd     `cmd:"" help:"Switch to a different Upbound profile."`
	View    viewCmd    `cmd:"" help:"View all Upbound profiles in JSON format."`
	Set     setCmd     `cmd:"" help:"Set configuration values for the current profile."`
	Create  createCmd  `cmd:"" help:"Create a new Upbound profile."`
	Delete  deleteCmd  `cmd:"" help:"Delete an existing Upbound profile."`
	Rename  renameCmd  `cmd:"" help:"Rename an existing Upbound profile."`
}

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *Cmd) AfterApply(kongCtx *kong.Context) error {
	// Let subcommands access the raw flags, in case they want to use different
	// defaults than the profile.
	kongCtx.Bind(c.Flags)
	return nil
}

// PredictProfiles is the completion predictor for profiles.
func PredictProfiles() complete.Predictor {
	return complete.PredictFunc(func(_ complete.Args) (prediction []string) {
		upCtx, err := upbound.NewFromFlags(upbound.Flags{})
		if err != nil {
			return nil
		}
		upCtx.SetupLogging()

		profiles, err := upCtx.Cfg.GetUpboundProfiles()
		if err != nil {
			return nil
		}

		data := make([]string, 0)

		for name := range profiles {
			data = append(data, name)
		}
		return data
	})
}
