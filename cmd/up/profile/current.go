// Copyright 2025 Upbound Inc.
// All rights reserved

package profile

import (
	"encoding/json"
	"fmt"

	"github.com/alecthomas/kong"

	"github.com/upbound/up/internal/profile"
	"github.com/upbound/up/internal/upbound"
)

type currentCmd struct{}

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
	fmt.Fprintln(ctx.Stdout, string(b))
	return nil
}
