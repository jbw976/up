// Copyright 2025 Upbound Inc.
// All rights reserved

package profile

import (
	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/upbound"
)

type renameCmd struct {
	From string `arg:"" help:"Name of the profile to rename." required:""`
	To   string `arg:"" help:"New name for the profile."      required:""`
}

func (c *renameCmd) Help() string {
	return `
The 'rename' command changes the name of an existing Upbound profile.

This command renames a profile while preserving all its configuration settings.
If the profile being renamed is currently active, it remains active after renaming.

The new name must not conflict with any existing profile names.

Usage Examples:
    up profile rename old-name new-name
        Renames the profile "old-name" to "new-name".

    up profile rename dev development
        Renames the profile "dev" to "development".
`
}

func (c *renameCmd) Run(upCtx *upbound.Context) error {
	if err := upCtx.Cfg.RenameUpboundProfile(c.From, c.To); err != nil {
		return err
	}

	return errors.Wrap(upCtx.CfgSrc.UpdateConfig(upCtx.Cfg), "unable to rename profile")
}
