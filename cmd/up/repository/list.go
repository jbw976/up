// Copyright 2025 Upbound Inc.
// All rights reserved

package repository

import (
	"context"
	"strconv"
	"time"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	"k8s.io/apimachinery/pkg/util/duration"

	"github.com/upbound/up-sdk-go/service/common"
	"github.com/upbound/up-sdk-go/service/repositories"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

const (
	maxItems = 100
)

// AfterApply sets default values in command after assignment and validation.
func (c *listCmd) AfterApply(kongCtx *kong.Context) error {
	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))
	return nil
}

// listCmd lists repositories in an account on Upbound.
type listCmd struct{}

//nolint:gochecknoglobals // Would make this a const if we could.
var fieldNames = []string{"NAME", "TYPE", "PUBLIC", "PUBLISH POLICY", "UPDATED"}

// Run executes the list command.
func (c *listCmd) Run(ctx context.Context, printer upterm.ObjectPrinter, p pterm.TextPrinter, rc *repositories.Client, upCtx *upbound.Context) error {
	rList, err := rc.List(ctx, upCtx.Organization, common.WithSize(maxItems))
	if err != nil {
		return err
	}
	if len(rList.Repositories) == 0 {
		p.Printfln("No repositories found in %s", upCtx.Organization)
		return nil
	}
	return printer.Print(rList.Repositories, fieldNames, extractFields)
}

func extractFields(obj any) []string {
	r := obj.(repositories.Repository) //nolint:forcetypeassert // Type assertion will always be true because of what's passed to printer.Print above.

	rt := "unknown"
	if r.Type != nil {
		rt = string(*r.Type)
	}
	u := "n/a"
	if r.UpdatedAt != nil {
		u = duration.HumanDuration(time.Since(*r.UpdatedAt))
	}
	return []string{r.Name, rt, strconv.FormatBool(r.Public), string(*r.Publish), u}
}
