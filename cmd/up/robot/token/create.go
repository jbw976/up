// Copyright 2025 Upbound Inc.
// All rights reserved

package token

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up-sdk-go/service/tokens"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

// createCmd creates a robot on Upbound.
type createCmd struct {
	RobotName string `arg:"" help:"Name of robot." required:""`
	TokenName string `arg:"" help:"Name of token." required:""`

	File string `help:"file to write Token JSON, Use '-' to write to standard output." short:"f"`
}

// Run executes the create command.
func (c *createCmd) Run(ctx context.Context, printer upterm.Printer, ac *accounts.Client, oc *organizations.Client, tc *tokens.Client, upCtx *upbound.Context) error {
	a, err := ac.Get(ctx, upCtx.Organization)
	if err != nil {
		return err
	}
	if a.Account.Type != accounts.AccountOrganization {
		return errors.New(errUserAccount)
	}
	rs, err := oc.ListRobots(ctx, a.Organization.ID)
	if err != nil {
		return err
	}
	if len(rs) == 0 {
		return errors.Errorf(errFindRobotFmt, c.RobotName, upCtx.Organization)
	}
	// TODO(hasheddan): because this API does not guarantee name uniqueness, we
	// must guarantee that exactly one robot exists in the specified account
	// with the provided name. Logic should be simplified when the API is
	// updated.
	var id uuid.UUID
	found := false
	for _, r := range rs {
		if r.Name == c.RobotName {
			if found {
				return errors.Errorf(errMultipleRobotFmt, c.RobotName, upCtx.Organization)
			}
			id = r.ID
			found = true
		}
	}
	if !found {
		return errors.Errorf(errFindRobotFmt, c.RobotName, upCtx.Organization)
	}
	res, err := tc.Create(ctx, &tokens.TokenCreateParameters{
		Attributes: tokens.TokenAttributes{
			Name: c.TokenName,
		},
		Relationships: tokens.TokenRelationships{
			Owner: tokens.TokenOwner{
				Data: tokens.TokenOwnerData{
					Type: tokens.TokenOwnerRobot,
					ID:   id.String(),
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

	printer.Printfln("%s/%s/%s created", upCtx.Organization, c.RobotName, c.TokenName)
	f, err := os.OpenFile(filepath.Clean(c.File), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck // Can't do anything useful with this error.
	return json.NewEncoder(f).Encode(tokenFile)
}
