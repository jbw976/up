// Copyright 2025 Upbound Inc.
// All rights reserved

package repository

import (
	"context"
	"strconv"
	"time"

	"k8s.io/apimachinery/pkg/util/duration"

	"github.com/upbound/up-sdk-go/service/common"
	"github.com/upbound/up-sdk-go/service/repositories"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

const (
	maxItems = 100
)

// listCmd lists repositories in an account on Upbound.
type listCmd struct{}

//nolint:gochecknoglobals // Would make this a const if we could.
var fieldNames = []string{"NAME", "TYPE", "PUBLIC", "PUBLISH POLICY", "UPDATED"}

// Run executes the list command.
func (c *listCmd) Run(ctx context.Context, printer upterm.Printer, rc *repositories.Client, upCtx *upbound.Context) error {
	rList, err := rc.List(ctx, upCtx.Organization, common.WithSize(maxItems))
	if err != nil {
		return err
	}
	if len(rList.Repositories) == 0 {
		printer.Printfln("No repositories found in %s", upCtx.Organization)
		return nil
	}
	return printer.PrintObject(rList.Repositories, fieldNames, extractFields)
}

func extractFields(obj any) []string {
	r, _ := obj.(repositories.Repository)

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
