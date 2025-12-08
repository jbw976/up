// Copyright 2025 Upbound Inc.
// All rights reserved

package uxp

import (
	"io"

	"github.com/pterm/pterm"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/install/helm"
	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/internal/uxp"
)

const (
	errParseUpgradeParameters = "unable to parse upgrade parameters"
)

// AfterApply sets default values in command after assignment and validation.
func (c *upgradeCmd) AfterApply(insCtx *install.Context) error {
	repo := uxp.RepoURL

	filter := uxp.StableVersionFilter
	if c.Unstable {
		filter = uxp.UnstableVersionFilter
	}

	ins, err := helm.NewManager(insCtx.Kubeconfig,
		uxp.ChartName,
		*repo,
		uxp.ChartNamespace,
		helm.WithChart(c.Bundle),
		helm.RollbackOnError(c.Rollback),
		helm.Force(c.Force),
		helm.Wait(),
		helm.CreateNamespace(true),
		helm.WithVersionFilter(filter),
	)
	if err != nil {
		return err
	}
	c.mgr = ins

	client, err := kubernetes.NewForConfig(insCtx.Kubeconfig)
	if err != nil {
		return err
	}
	c.kClient = client

	values := uxp.BaseValues()
	if c.File != nil {
		defer func() { _ = c.File.Close() }()
		b, err := io.ReadAll(c.File)
		if err != nil {
			return errors.Wrap(err, errReadParametersFile)
		}
		if err := yaml.Unmarshal(b, &values); err != nil {
			return errors.Wrap(err, errReadParametersFile)
		}
		if err := c.File.Close(); err != nil {
			return errors.Wrap(err, errReadParametersFile)
		}
	}

	c.parser = helm.NewParser(values, c.Set)
	return nil
}

// upgradeCmd upgrades UXP.
type upgradeCmd struct {
	mgr     install.Manager
	parser  install.ParameterParser
	kClient kubernetes.Interface

	Version string `arg:"" help:"UXP version to upgrade to." optional:""`

	Rollback bool `help:"Rollback to previously installed version on failed upgrade."`
	Force    bool `help:"Force upgrade even if versions are incompatible."`
	Unstable bool `help:"Allow installing unstable versions."`

	install.CommonParams
}

// Run executes the upgrade command.
func (c *upgradeCmd) Run(p upterm.ObjectPrinter) error {
	if err := upterm.WrapWithSuccessSpinner(
		"Upgrading UXP",
		func() error {
			params, err := c.parser.Parse()
			if err != nil {
				return errors.Wrap(err, errParseUpgradeParameters)
			}
			return c.mgr.Upgrade(c.Version, params)
		},
		p,
	); err != nil {
		return err
	}

	curVer, err := c.mgr.GetCurrentVersion()
	if err != nil {
		return err
	}
	pterm.Info.WithPrefix(upterm.RaisedPrefix).Printfln("UXP upgraded to %s", curVer)
	return nil
}
