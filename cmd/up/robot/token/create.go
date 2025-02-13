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
	"github.com/pterm/pterm"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up-sdk-go/service/tokens"
	"github.com/upbound/up/internal/upbound"
)

// createCmd creates a robot on Upbound.
type createCmd struct {
	RobotName string `arg:"" help:"Name of robot." required:""`
	TokenName string `arg:"" help:"Name of token." required:""`

	Output string `help:"Path to write JSON file containing access ID and token." required:"" short:"o" type:"path"`
}

// Run executes the create command.
func (c *createCmd) Run(ctx context.Context, p pterm.TextPrinter, ac *accounts.Client, oc *organizations.Client, tc *tokens.Client, upCtx *upbound.Context) error {
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
	p.Printfln("%s/%s/%s created", upCtx.Organization, c.RobotName, c.TokenName)
	if c.Output == "" {
		p.Printfln("Refusing to emit sensitive output. Please specify output location.")
		return nil
	}

	access := res.ID.String()
	token := fmt.Sprint(res.DataSet.Meta["jwt"])
	if c.Output == "-" {
		pterm.Println()
		p.Printfln(pterm.LightMagenta("Access ID: ") + access)
		p.Printfln(pterm.LightMagenta("Token: ") + token)
		return nil
	}

	f, err := os.OpenFile(filepath.Clean(c.Output), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck // Can't do anything useful with this error.
	return json.NewEncoder(f).Encode(&upbound.TokenFile{
		AccessID: access,
		Token:    token,
	})
}
