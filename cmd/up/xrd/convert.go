// Copyright 2025 Upbound Inc.
// All rights reserved

package xrd

import (
	"context"

	"github.com/alecthomas/kong"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/upbound/up/internal/crd"
	"github.com/upbound/up/internal/upterm"

	_ "embed"
)

//go:embed help/convert.md
var convertHelp string

type convertCmd struct {
	File      string `arg:""      help:"Path to the XRD file to convert."`
	OutputDir string `default:"." help:"Directory where the generated CRD files will be saved." short:"o"`

	fs afero.Fs
}

func (c *convertCmd) Help() string {
	return convertHelp
}

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *convertCmd) AfterApply(kongCtx *kong.Context) error {
	ctx := context.Background()

	c.fs = afero.NewOsFs()

	// workaround interfaces not being bindable ref: https://github.com/alecthomas/kong/issues/48
	kongCtx.BindTo(ctx, (*context.Context)(nil))
	return nil
}

func (c *convertCmd) Run(p upterm.Printer) error {
	// Read the XRD file
	xrdData, err := afero.ReadFile(c.fs, c.File)
	if err != nil {
		return errors.Wrapf(err, "failed to read XRD file %s", c.File)
	}

	// Process the XRD to generate CRDs
	xrPath, claimPath, err := crd.ProcessXRD(c.fs, xrdData, "", c.OutputDir)
	if err != nil {
		return errors.Wrap(err, "failed to convert XRD to CRDs")
	}

	if xrPath == "" && claimPath == "" {
		return errors.New("no CRDs were generated from the XRD")
	}

	p.Printfln("Successfully converted XRD to CRD(s)")

	return nil
}
