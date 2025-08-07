// Copyright 2025 Upbound Inc.
// All rights reserved

package profile

import (
	"encoding/json"
	"fmt"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"

	"github.com/upbound/up/internal/profile"
	"github.com/upbound/up/internal/upbound"

	_ "embed"
)

type viewCmd struct{}

//go:embed help/view.md
var viewHelp string

func (c *viewCmd) Help() string {
	return viewHelp
}

// Run executes the list command.
func (c *viewCmd) Run(p pterm.TextPrinter, ctx *kong.Context, upCtx *upbound.Context) error {
	profiles, err := upCtx.Cfg.GetUpboundProfiles()
	if err != nil {
		p.Println("No profiles found")
		return nil //nolint:nilerr // error is handled by printing message
	}

	redacted := make(map[string]profile.Redacted)
	for k, v := range profiles {
		redacted[k] = profile.Redacted{Profile: v}
	}

	b, err := json.MarshalIndent(redacted, "", "    ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(ctx.Stdout, string(b))
	return err
}
