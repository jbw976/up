// Copyright 2025 Upbound Inc.
// All rights reserved

package profile

import (
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/upbound/up/internal/upbound"

	_ "embed"
)

type deleteCmd struct {
	Name string `arg:"" help:"Name of the profile to delete." required:""`
}

//go:embed help/delete.md
var deleteHelp string

func (c *deleteCmd) Help() string {
	return deleteHelp
}

func (c *deleteCmd) Run(upCtx *upbound.Context) error {
	if err := upCtx.Cfg.DeleteUpboundProfile(c.Name); err != nil {
		return err
	}

	return errors.Wrap(upCtx.CfgSrc.UpdateConfig(upCtx.Cfg), "unable to delete profile")
}
