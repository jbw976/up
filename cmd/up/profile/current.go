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

type currentCmd struct{}

//go:embed help/current.md
var currentHelp string

func (c *currentCmd) Help() string {
	return currentHelp
}

type output struct {
	Name    string           `json:"name"`
	Profile profile.Redacted `json:"profile"`
}

// Run executes the current command.
func (c *currentCmd) Run(upCtx *upbound.Context, p upterm.Printer) error {
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

	// TODO(adamwg): Use PrintObject to respect the format flag.
	p.PrintResult(string(b))

	return err
}
