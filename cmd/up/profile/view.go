// Copyright 2025 Upbound Inc.
// All rights reserved

package profile

import (
	"encoding/json"

	"github.com/upbound/up/internal/profile"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"

	_ "embed"
)

type viewCmd struct{}

//go:embed help/view.md
var viewHelp string

func (c *viewCmd) Help() string {
	return viewHelp
}

// Run executes the list command.
func (c *viewCmd) Run(p upterm.Printer, upCtx *upbound.Context) error {
	profiles, err := upCtx.Cfg.GetUpboundProfiles()
	if err != nil {
		p.Println("No profiles found")
		return nil //nolint:nilerr // error is handled by printing message
	}

	redacted := make(map[string]profile.Redacted)
	for k, v := range profiles {
		redacted[k] = profile.Redacted{Profile: v}
	}

	// TODO(adamwg): Should we respect use p.PrintObject so we respect the
	// format flag here?
	b, err := json.MarshalIndent(redacted, "", "    ")
	if err != nil {
		return err
	}

	p.PrintResult(string(b))

	return nil
}
