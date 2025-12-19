// Copyright 2025 Upbound Inc.
// All rights reserved

package token

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/util/duration"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/common"
	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up-sdk-go/service/robots"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

//nolint:gochecknoglobals // Would make this a const if we could.
var fieldNames = []string{"NAME", "ID", "CREATED"}

// listCmd creates a robot on Upbound.
type listCmd struct {
	RobotName string `arg:"" help:"Name of robot." predictor:"robots" required:""`
}

// Run executes the list robot tokens command.
func (c *listCmd) Run(ctx context.Context, printer upterm.Printer, ac *accounts.Client, oc *organizations.Client, rc *robots.Client, upCtx *upbound.Context) error {
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
	var rid *uuid.UUID
	for _, r := range rs {
		if r.Name == c.RobotName {
			if rid != nil {
				return errors.Errorf(errMultipleRobotFmt, c.RobotName, upCtx.Organization)
			}
			// Pin range variable so that we can take address.
			r := r
			rid = &r.ID
		}
	}
	if rid == nil {
		return errors.Errorf(errFindRobotFmt, c.RobotName, upCtx.Organization)
	}

	ts, err := rc.ListTokens(ctx, *rid)
	if err != nil {
		return err
	}
	if len(ts.DataSet) == 0 {
		printer.Printfln("No tokens found for robot %s in %s", c.RobotName, upCtx.Organization)
		return nil
	}
	return printer.PrintObject(ts.DataSet, fieldNames, extractFields)
}

func extractFields(obj any) []string {
	t := obj.(common.DataSet) //nolint:forcetypeassert // Type assertion will always be true because of what's passed to printer.Print above.

	n := fmt.Sprint(t.AttributeSet["name"])
	c := "n/a"
	if ca, ok := t.Meta["createdAt"]; ok {
		if ct, err := time.Parse(time.RFC3339, fmt.Sprint(ca)); err == nil {
			c = duration.HumanDuration(time.Since(ct))
		}
	}
	return []string{n, t.ID.String(), c}
}
