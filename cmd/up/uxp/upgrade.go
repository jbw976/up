// Copyright 2025 Upbound Inc.
// All rights reserved

package uxp

import (
	"io"

	"github.com/pterm/pterm"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/install/helm"
)

const (
	errParseUpgradeParameters = "unable to parse upgrade parameters"
)

// AfterApply sets default values in command after assignment and validation.
func (c *upgradeCmd) AfterApply(insCtx *install.Context) error {
	repo := RepoURL
	if c.Unstable {
		repo = uxpUnstableRepoURL
	}
	ins, err := helm.NewManager(insCtx.Kubeconfig,
		chartName,
		repo,
		helm.WithNamespace(insCtx.Namespace),
		helm.WithChart(c.Bundle),
		helm.WithAlternateChart(alternateChartName),
		helm.RollbackOnError(c.Rollback),
		helm.Force(c.Force))
	if err != nil {
		return err
	}
	c.mgr = ins
	base := map[string]any{}
	if c.File != nil {
		defer c.File.Close() //nolint:errcheck,gosec
		b, err := io.ReadAll(c.File)
		if err != nil {
			return errors.Wrap(err, errReadParametersFile)
		}
		if err := yaml.Unmarshal(b, &base); err != nil {
			return errors.Wrap(err, errReadParametersFile)
		}
		if err := c.File.Close(); err != nil {
			return errors.Wrap(err, errReadParametersFile)
		}
	}
	c.parser = helm.NewParser(base, c.Set)
	return nil
}

// upgradeCmd upgrades UXP.
type upgradeCmd struct {
	mgr    install.Manager
	parser install.ParameterParser

	Version string `arg:"" help:"UXP version to upgrade to." optional:""`

	Rollback bool `help:"Rollback to previously installed version on failed upgrade."`
	Force    bool `help:"Force upgrade even if versions are incompatible."`
	Unstable bool `help:"Allow installing unstable versions."`

	install.CommonParams
}

// Run executes the upgrade command.
func (c *upgradeCmd) Run(p pterm.TextPrinter, insCtx *install.Context) error {
	params, err := c.parser.Parse()
	if err != nil {
		return errors.Wrap(err, errParseUpgradeParameters)
	}
	if err := c.mgr.Upgrade(c.Version, params); err != nil {
		return err
	}
	curVer, err := c.mgr.GetCurrentVersion()
	if err != nil {
		return err
	}
	p.Printfln("UXP upgraded to %s", curVer)
	return nil
}
