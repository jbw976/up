// Copyright 2025 Upbound Inc.
// All rights reserved

package uxp

import (
	"context"
	"fmt"
	"io"

	"github.com/pterm/pterm"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/install/helm"
	"github.com/upbound/up/internal/registry"
	"github.com/upbound/up/internal/registry/pullsecret"
	"github.com/upbound/up/internal/upterm"
)

const (
	errReadParametersFile     = "unable to read parameters file"
	errParseInstallParameters = "unable to parse install parameters"
	errCreateImagePullSecret  = "failed to create image pull secret"
)

// AfterApply sets default values in command after assignment and validation.
func (c *installCmd) AfterApply(insCtx *install.Context) error {
	repo := RepoURL
	if c.Unstable {
		repo = uxpUnstableRepoURL
	}
	mgr, err := helm.NewManager(insCtx.Kubeconfig,
		chartName,
		*repo,
		chartNamespace,
		helm.WithChart(c.Bundle))
	if err != nil {
		return err
	}
	c.mgr = mgr
	client, err := kubernetes.NewForConfig(insCtx.Kubeconfig)
	if err != nil {
		return err
	}
	c.kClient = client
	values := baseValues()
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

// installCmd installs UXP.
type installCmd struct {
	mgr     install.Manager
	parser  install.ParameterParser
	kClient kubernetes.Interface

	Version  string `arg:""                                     help:"UXP version to install." optional:""`
	Unstable bool   `help:"Allow installing unstable versions."`

	Registry registry.AuthorizedFlags `embed:""`
	install.CommonParams
}

// Run executes the install command.
func (c *installCmd) Run(ctx context.Context, p upterm.ObjectPrinter) error {
	if err := c.Registry.AfterApply(); err != nil {
		return err
	}

	// TODO(branden): Remove this once UXP is public.
	pullSecret := pullsecret.NewManagerFromFlags(c.kClient, imagePullSecret, chartNamespace, c.Registry)

	if err := upterm.WrapWithSuccessSpinner(
		upterm.StepCounter(fmt.Sprintf("Creating pull secret %s", imagePullSecret), 1, 2),
		upterm.CheckmarkSuccessSpinner,
		func() error {
			return errors.Wrap(pullSecret.CreateOrUpdate(ctx), errCreateImagePullSecret)
		},
		p,
	); err != nil {
		return err
	}

	if err := upterm.WrapWithSuccessSpinner(
		upterm.StepCounter("Installing UXP", 2, 2),
		upterm.CheckmarkSuccessSpinner,
		func() error {
			params, err := c.parser.Parse()
			if err != nil {
				return errors.Wrap(err, errParseInstallParameters)
			}
			return c.mgr.Install(c.Version, params)
		},
		p,
	); err != nil {
		return err
	}

	curVer, err := c.mgr.GetCurrentVersion()
	if err != nil {
		return err
	}
	pterm.Info.WithPrefix(upterm.RaisedPrefix).Printfln("UXP %s installed", curVer)
	return nil
}
