// Copyright 2025 Upbound Inc.
// All rights reserved

package profile

import (
	"encoding/json"
	"fmt"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"

	"github.com/upbound/up/internal/profile"
	"github.com/upbound/up/internal/style"
	"github.com/upbound/up/internal/upbound"
)

type viewCmd struct{}

func (c *viewCmd) Help() string {
	return style.RenderHelp(`
The <view> command displays all configured Upbound profiles in JSON format.

This command outputs detailed information about all profiles, including:
- Profile names as keys
- Profile configuration details (with sensitive data redacted)
- Profile type, organization, domain, and other settings

The output is formatted as indented JSON for easy reading and processing.

## Usage Examples:

    up profile view
        Shows all profiles in JSON format.

    up profile view | jq '.["my-profile"]'
        Shows only the "my-profile" configuration using jq.
`)
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
