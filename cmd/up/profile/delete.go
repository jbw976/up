// Copyright 2025 Upbound Inc.
// All rights reserved

package profile

import (
	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/style"
	"github.com/upbound/up/internal/upbound"
)

type deleteCmd struct {
	Name string `arg:"" help:"Name of the profile to delete." required:""`
}

func (c *deleteCmd) Help() string {
	return style.RenderHelp(`
The <delete> command removes an Upbound profile from the configuration.

This command permanently deletes the specified profile and all its associated configuration.
The profile cannot be recovered after deletion.

Note: You cannot delete the currently active profile. Switch to a different profile first
using 'up profile use' if you need to delete the active profile.

## Usage Examples:

    up profile delete <old-profile>
        Deletes the profile named "old-profile".

    up profile delete <staging>
        Deletes the profile named "staging".
`)
}

func (c *deleteCmd) Run(upCtx *upbound.Context) error {
	if err := upCtx.Cfg.DeleteUpboundProfile(c.Name); err != nil {
		return err
	}

	return errors.Wrap(upCtx.CfgSrc.UpdateConfig(upCtx.Cfg), "unable to delete profile")
}
