// Copyright 2025 Upbound Inc.
// All rights reserved

package profile

import (
	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/upbound"
)

type setCmd struct {
	Key   string `arg:"" enum:"organization,domain"             help:"The configuration key to set." required:""`
	Value string `arg:"" help:"The configuration value to set." required:""`
}

func (c *setCmd) Run(upCtx *upbound.Context) error {
	switch c.Key {
	case "organization":
		upCtx.Profile.Organization = c.Value
		// Clear the deprecated account field if organization is set.
		upCtx.Profile.Account = "" //nolint:staticcheck // Migration off deprecated value.

	case "domain":
		upCtx.Profile.Domain = c.Value

	default:
		// Should never hit this due to kong validation.
		return errors.New("invalid key")
	}

	if err := upCtx.Cfg.AddOrUpdateUpboundProfile(upCtx.ProfileName, upCtx.Profile); err != nil {
		return err
	}

	return errors.Wrap(upCtx.CfgSrc.UpdateConfig(upCtx.Cfg), errUpdateProfile)
}
