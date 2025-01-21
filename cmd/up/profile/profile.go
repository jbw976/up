// Copyright 2022 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package profile contains commands for working with configuration profiles.
package profile

import (
	"github.com/alecthomas/kong"
	"github.com/posener/complete"

	"github.com/upbound/up/cmd/up/profile/config"
	"github.com/upbound/up/internal/upbound"
)

// Cmd contains commands for Upbound Profiles.
type Cmd struct {
	Current currentCmd `cmd:"" help:"Get current Upbound Profile."`
	List    listCmd    `cmd:"" help:"List Upbound Profiles."`
	Use     useCmd     `cmd:"" help:"Select an Upbound Profile as the default."`
	View    viewCmd    `cmd:"" help:"View the Upbound Profile settings across profiles."`
	Set     setCmd     `cmd:"" help:"Set Upbound Profile parameters."`
	Create  createCmd  `cmd:"" help:"Create a new Upbound Profile without logging in."`
	Delete  deleteCmd  `cmd:"" help:"Delete an Upbound Profile."`
	Rename  renameCmd  `cmd:"" help:"Rename an Upbound Profile."`
	Config  config.Cmd `cmd:"" deprecated:""                                             help:"Deprecated: Interact with the current Upbound Profile's config." hidden:""`

	Flags upbound.Flags `embed:""`
}

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *Cmd) AfterApply(kongCtx *kong.Context) error {
	upCtx, err := upbound.NewFromFlags(c.Flags, upbound.AllowMissingProfile())
	if err != nil {
		return err
	}
	upCtx.SetupLogging()

	// Let subcommands access the raw flags, in case they want to use different
	// defaults than the profile.
	kongCtx.Bind(upCtx, c.Flags)
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
