// Copyright 2025 Upbound Inc.
// All rights reserved

package login

import (
	"context"
	"net/http"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up-sdk-go"
	"github.com/upbound/up/internal/upbound"
)

const (
	logoutPath = "/v1/logout"

	errLogoutFailed      = "unable to logout"
	errRemoveTokenFailed = "failed to remove token"
)

// AfterApply sets default values in login after assignment and validation.
func (c *LogoutCmd) AfterApply(kongCtx *kong.Context) error {
	upCtx, err := upbound.NewFromFlags(c.Flags)
	if err != nil {
		return err
	}
	upCtx.SetupLogging()

	kongCtx.Bind(upCtx)

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

	// Common Upbound API configuration
	Flags upbound.Flags `embed:""`
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
