// Copyright 2025 Upbound Inc.
// All rights reserved

package profile

import (
	"sort"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"

	"github.com/upbound/up/internal/profile"
	"github.com/upbound/up/internal/upbound"
)

// AfterApply sets default values in command after assignment and validation.
func (c *listCmd) AfterApply(kongCtx *kong.Context) error {
	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))
	return nil
}

type listCmd struct{}

// Run executes the list command.
func (c *listCmd) Run(p pterm.TextPrinter, pt *pterm.TablePrinter, upCtx *upbound.Context) error {
	profiles, err := upCtx.Cfg.GetUpboundProfiles()
	if err != nil {
		p.Println(errNoProfiles)
		return nil //nolint:nilerr // Successfully list nothing if there are no profiles.
	}

	redacted := make(map[string]profile.Redacted)
	for k, v := range profiles {
		redacted[k] = profile.Redacted{Profile: v}
	}
	if len(redacted) == 0 {
		p.Println(errNoProfiles)
		return nil
	}

	// sort the redacted profiles by name so that we have a consistent listing
	profileNames := make([]string, 0, len(redacted))
	for k := range redacted {
		profileNames = append(profileNames, k)
	}
	sort.Strings(profileNames)

	dprofile, _, err := upCtx.Cfg.GetDefaultUpboundProfile()
	if err != nil {
		return err
	}

	data := make([][]string, len(redacted)+1)
	cursor := ""

	data[0] = []string{"CURRENT", "NAME", "TYPE", "ORGANIZATION"}
	for i, name := range profileNames {
		if name == dprofile {
			cursor = "*"
		}
		prof := redacted[name]
		data[i+1] = []string{cursor, name, string(prof.Type), prof.Organization}

		cursor = "" // reset cursor
	}

	return pt.WithHasHeader().WithData(data).Render()
}
