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

func (c *renameCmd) Run(upCtx *upbound.Context) error {
	if err := upCtx.Cfg.RenameUpboundProfile(c.From, c.To); err != nil {
		return err
	}

	return errors.Wrap(upCtx.CfgSrc.UpdateConfig(upCtx.Cfg), "unable to rename profile")
}
