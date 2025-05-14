// Copyright 2025 Upbound Inc.
// All rights reserved

// Package token contains commands for working with personal user tokens.
package token

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/tokens"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

// createCmd creates a personal access token on Upbound for the current user.
type createCmd struct {
	TokenName string `arg:"" help:"Name of token." required:""`

	File string `help:"file to write Token JSON, Use '-' to write to standard output." short:"f"`
}

// AfterApply sets default values in command after assignment and validation.
func (c *createCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))
	kongCtx.Bind(upCtx)

	return nil
}

// Run executes the create command.
func (c *createCmd) Run(ctx context.Context, p pterm.TextPrinter, printer upterm.ObjectPrinter, ac *accounts.Client, tc *tokens.Client, upCtx *upbound.Context) error {
	tokenFields := []string{
		"AccessID",
		"Token",
	}

	a, err := ac.Get(ctx, upCtx.Organization)
	if err != nil {
		return err
	}
	if a.Account.Type != accounts.AccountOrganization {
		return errors.New(errRobot)
	}

	// get the userID
	u, err := ac.Get(ctx, upCtx.Profile.ID)
	if err != nil {
		return err
	}

	res, err := tc.Create(ctx, &tokens.TokenCreateParameters{
		Attributes: tokens.TokenAttributes{
			Name: c.TokenName,
		},
		Relationships: tokens.TokenRelationships{
			Owner: tokens.TokenOwner{
				Data: tokens.TokenOwnerData{
					Type: tokens.TokenOwnerUser,
					ID:   strconv.FormatUint(uint64(u.Organization.CreatorID), 10),
				},
			},
		},
	})
	if err != nil {
		return err
	}

	if c.File == "" {
		p.Printfln("Refusing to emit sensitive output. Please specify file location.")
		return nil
	}

	tokenFile := &upbound.TokenFile{
		AccessID: res.ID.String(),
		Token:    fmt.Sprint(res.DataSet.Meta["jwt"]),
	}
	if c.File == "-" {
		// print token always as json
		printer.Format = "json"
		return printer.Print(tokenFile, tokenFields, extractTokenFields)
	}

	p.Printfln("%s/%s created", upCtx.Profile.ID, c.TokenName)
	f, err := os.OpenFile(filepath.Clean(c.File), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck // Can't do anything useful with this error.
	return json.NewEncoder(f).Encode(tokenFile)
}

// Define the extract function for tokenFile.
func extractTokenFields(obj any) []string {
	t, ok := obj.(*upbound.TokenFile)
	if !ok || t == nil {
		return []string{}
	}
	return []string{t.AccessID, t.Token}
}
