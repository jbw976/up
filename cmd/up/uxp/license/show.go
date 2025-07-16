// Copyright 2025 Upbound Inc.
// All rights reserved

package license

import (
	"context"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/upbound/up/internal/license"
	"github.com/upbound/up/internal/upterm"

	_ "embed"
)

//go:embed show.tmpl
var tmpl string

// showCmd is the `up uxp license show` command.
type showCmd struct{}

// Run is the body of the command.
func (c *showCmd) Run(cl client.Client, printer upterm.ObjectPrinter) error {
	l, err := license.FromUXPv2(context.Background(), cl)
	if err != nil {
		return errors.Wrap(err, "failed to get license")
	}

	if err := printer.PrintTemplate(l, tmpl); err != nil {
		return errors.Wrap(err, "failed to show license")
	}

	return nil
}
