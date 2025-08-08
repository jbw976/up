// Copyright 2025 Upbound Inc.
// All rights reserved

package login

import (
	"context"
	"net/http"

	"github.com/pterm/pterm"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up-sdk-go"
	"github.com/upbound/up/internal/style"
	"github.com/upbound/up/internal/upbound"
)

const (
	logoutPath = "/v1/logout"

	errLogoutFailed      = "unable to logout"
	errRemoveTokenFailed = "failed to remove token"
)

// AfterApply sets default values in login after assignment and validation.
func (c *LogoutCmd) AfterApply(upCtx *upbound.Context) error {
	cfg, err := upCtx.BuildSDKConfig()
	if err != nil {
		return err
	}
	c.client = cfg.Client
	return nil
}

// LogoutCmd invalidates a stored session token for a given profile.
type LogoutCmd struct {
	client up.Client
}

// Help returns help text for the logout command.
func (c *LogoutCmd) Help() string {
	return style.RenderHelp(`
The <logout> command invalidates the current session and removes stored credentials.

This command:
  - Invalidates the session token with Upbound Cloud
  - Removes the session token from the local profile configuration
  - Keeps the profile configuration intact (only removes authentication)

After logout, you can log back in using 'up login' to re-authenticate with the same profile.

## Usage Examples:

    up logout
        Logs out from the selected active profile.

    up logout --profile=<production>
        Logs out from the "production" profile.

*Note*: This only affects the selected profile. Other profiles remain authenticated.
`)
}

// Run executes the logout command.
func (c *LogoutCmd) Run(ctx context.Context, p pterm.TextPrinter, upCtx *upbound.Context) error {
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	req, err := c.client.NewRequest(ctx, http.MethodPost, logoutPath, "", nil)
	if err != nil {
		return errors.Wrap(err, errLogoutFailed)
	}
	if err := c.client.Do(req, nil); err != nil {
		return errors.Wrap(err, errLogoutFailed)
	}
	// Logout is successful, remove token from config and update.
	upCtx.Profile.Session = ""
	if err := upCtx.Cfg.AddOrUpdateUpboundProfile(upCtx.ProfileName, upCtx.Profile); err != nil {
		return errors.Wrap(err, errRemoveTokenFailed)
	}
	if err := upCtx.CfgSrc.UpdateConfig(upCtx.Cfg); err != nil {
		return errors.Wrap(err, errUpdateConfig)
	}

	p.Printfln("%s logged out", upCtx.Profile.ID)
	return nil
}
