// Copyright 2025 Upbound Inc.
// All rights reserved

package token

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/upbound/up-sdk-go/service/tokens"
	"github.com/upbound/up-sdk-go/service/userinfo"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

// createCmd creates a personal access token on Upbound for the current user.
type createCmd struct {
	TokenName string `arg:"" help:"Name of token." required:""`

	File string `help:"file to write Token JSON, Use '-' to write to standard output." short:"f"`
}

// Run executes the create command.
func (c *createCmd) Run(ctx context.Context, printer upterm.Printer, ui *userinfo.Client, tc *tokens.Client, upCtx *upbound.Context) error {
	// get the userID
	u, err := ui.Get(ctx)
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
					ID:   strconv.FormatUint(uint64(u.User.ID), 10),
				},
			},
		},
	})
	if err != nil {
		return err
	}

	if c.File == "" {
		printer.Printfln("Refusing to emit sensitive output. Please specify file location.")
		return nil
	}

	tokenFile := &upbound.TokenFile{
		AccessID: res.ID.String(),
		Token:    fmt.Sprint(res.Meta["jwt"]),
	}
	if c.File == "-" {
		// print token always as json
		return json.NewEncoder(os.Stdout).Encode(tokenFile)
	}

	printer.Printfln("%s/%s created", upCtx.Profile.ID, c.TokenName)
	f, err := os.OpenFile(filepath.Clean(c.File), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck // Can't do anything useful with this error.
	return json.NewEncoder(f).Encode(tokenFile)
}
