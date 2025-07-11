// Copyright 2025 Upbound Inc.
// All rights reserved

package license

import (
	"context"

	"k8s.io/apimachinery/pkg/types"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/controller-manager/apis/licensing/v1alpha1"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"

	_ "embed"
)

//go:embed show.tmpl
var tmpl string

// showCmd is the `up uxp license show` command.
type showCmd struct{}

// Run is the body of the command.
func (c *showCmd) Run(upCtx *upbound.Context, printer upterm.ObjectPrinter) error {
	cl, err := upCtx.BuildCurrentContextClient()
	if err != nil {
		return errors.Wrap(err, "failed to get kube client")
	}

	if err := v1alpha1.AddToScheme(cl.Scheme()); err != nil {
		return errors.Wrap(err, "failed to add license types to scheme")
	}

	var l v1alpha1.License
	if err := cl.Get(context.Background(), types.NamespacedName{Name: v1alpha1.LicenseName}, &l); err != nil {
		return errors.Wrap(err, "failed to get license")
	}

	if err := printer.PrintTemplate(l, tmpl); err != nil {
		return errors.Wrap(err, "failed to show license")
	}

	return nil
}
