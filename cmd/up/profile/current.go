// Copyright 2025 Upbound Inc.
// All rights reserved

package profile

import (
	"encoding/json"
	"fmt"

	"github.com/alecthomas/kong"

	"github.com/upbound/up/internal/profile"
	"github.com/upbound/up/internal/style"
	"github.com/upbound/up/internal/upbound"
)

type currentCmd struct{}

func (c *currentCmd) Help() string {
	return style.RenderHelp(`
The <current> command displays the currently active Upbound profile and its configuration.

This command outputs JSON-formatted information about the active profile, including:
- Profile name
- Profile type (cloud or disconnected)
- Organization (for cloud profiles)
- Domain configuration
- Other profile settings (with sensitive data redacted)

## Usage Examples:

    up profile current
        Shows the current active profile configuration in JSON format.
`)
}

type output struct {
	Name    string           `json:"name"`
	Profile profile.Redacted `json:"profile"`
}

// Run executes the current command.
func (c *currentCmd) Run(ctx *kong.Context, upCtx *upbound.Context) error {
	name, prof, err := upCtx.Cfg.GetDefaultUpboundProfile()
	if err != nil {
		return err
	}

	redacted := profile.Redacted{Profile: prof}

	b, err := json.MarshalIndent(output{
		Name:    name,
		Profile: redacted,
	}, "", "    ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(ctx.Stdout, string(b))
	return err
}
