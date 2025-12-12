// Copyright 2025 Upbound Inc.
// All rights reserved

package profile

import (
	"sort"

	"github.com/upbound/up/internal/profile"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"

	_ "embed"
)

type listCmd struct{}

//go:embed help/list.md
var listHelp string

func (c *listCmd) Help() string {
	return listHelp
}

// Run executes the list command.
func (c *listCmd) Run(p upterm.Printer, upCtx *upbound.Context) error {
	profiles, err := upCtx.Cfg.GetUpboundProfiles()
	if err != nil {
		p.Println("No profiles found")
		return nil //nolint:nilerr // Successfully list nothing if there are no profiles.
	}

	redacted := make(map[string]profile.Redacted)
	for k, v := range profiles {
		redacted[k] = profile.Redacted{Profile: v}
	}
	if len(redacted) == 0 {
		p.Println("No profiles found")
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

	data := make([][]string, len(redacted))
	cursor := ""

	fieldNames := []string{"CURRENT", "NAME", "TYPE", "ORGANIZATION"}
	for i, name := range profileNames {
		if name == dprofile {
			cursor = "*"
		}
		prof := redacted[name]
		data[i] = []string{cursor, name, string(prof.Type), prof.Organization}

		cursor = "" // reset cursor
	}

	return p.PrintObject(data, fieldNames, extractFields)
}

func extractFields(p any) []string {
	return p.([]string) //nolint:forcetypeassert // Constructed above.
}
