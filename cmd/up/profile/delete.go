// Copyright 2025 Upbound Inc.
// All rights reserved

package profile

import (
	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/upbound"
)

type deleteCmd struct {
	Name string `arg:"" help:"Name of the profile to delete." required:""`
}

func (c *deleteCmd) Run(upCtx *upbound.Context) error {
	if err := upCtx.Cfg.DeleteUpboundProfile(c.Name); err != nil {
		return err
	}

	return errors.Wrap(upCtx.CfgSrc.UpdateConfig(upCtx.Cfg), "unable to delete profile")
}
