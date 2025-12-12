// Copyright 2025 Upbound Inc.
// All rights reserved

package token

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/duration"

	"github.com/upbound/up-sdk-go/service/common"
	"github.com/upbound/up-sdk-go/service/userinfo"
	"github.com/upbound/up-sdk-go/service/users"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

//nolint:gochecknoglobals // Would make this a const if we could.
var fieldNames = []string{"NAME", "ID", "CREATED"}

// listCmd list all tokens from current user.
type listCmd struct{}

// Run executes the list personal access tokens command.
func (c *listCmd) Run(ctx context.Context, printer upterm.Printer, ui *userinfo.Client, uc *users.Client, upCtx *upbound.Context) error {
	// get the userID
	u, err := ui.Get(ctx)
	if err != nil {
		return err
	}

	ts, err := uc.ListTokens(ctx, u.User.ID)
	if err != nil {
		return err
	}
	if len(ts.DataSet) == 0 {
		printer.Printfln("No personal access tokens found for user %s", upCtx.Profile.ID)
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
