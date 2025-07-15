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

func (c *setCmd) Help() string {
	return `
The 'set' command updates configuration values for the current Upbound profile.

Available configuration keys:
    organization - Sets the default organization for the current profile
    domain       - Sets the Upbound API domain for the current profile

Usage Examples:
    up profile set organization my-org
        Sets the default organization to "my-org" for the current profile.

    up profile set domain api.upbound.io
        Sets the Upbound API domain to "api.upbound.io" for the current profile.
`
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
