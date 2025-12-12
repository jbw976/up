// Copyright 2025 Upbound Inc.
// All rights reserved

package space

import (
	"context"
	"net/http"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	upboundv1alpha1 "github.com/upbound/up-sdk-go/apis/upbound/v1alpha1"
	uerrors "github.com/upbound/up-sdk-go/errors"
	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

const (
	errListSpaces = "unable to list Upbound Spaces"
)

// listCmd lists all of the spaces in Upbound.
type listCmd struct {
	upbound.RequiresContext

	kc client.Client
	ac *accounts.Client
}

// AfterApply sets default values in command after assignment and validation.
func (c *listCmd) AfterApply(upCtx *upbound.Context) error {
	cfg, err := upCtx.BuildSDKConfig()
	if err != nil {
		return err
	}

	ctrlCfg, err := upCtx.BuildControllerClientConfig()
	if err != nil {
		return err
	}
	c.ac = accounts.NewClient(cfg)

	kc, err := client.New(ctrlCfg, client.Options{})
	if err != nil {
		return errors.Wrap(err, errListSpaces)
	}
	c.kc = kc

	return nil
}

// Run executes the list command.
func (c *listCmd) Run(ctx context.Context, printer upterm.Printer, upCtx *upbound.Context) error {
	a, err := upbound.GetOrganization(ctx, c.ac, upCtx.Organization)
	var uerr *uerrors.Error
	if errors.As(err, &uerr) {
		if uerr.Status == http.StatusUnauthorized {
			printer.Println("You must be logged in and authorized to list Upbound Cloud Spaces")
			return uerr
		}
	}

	if err != nil {
		return errors.Wrap(err, errListSpaces)
	}

	var l upboundv1alpha1.SpaceList
	err = c.kc.List(ctx, &l, &client.ListOptions{Namespace: a.Organization.Name})
	if err != nil {
		return errors.Wrap(err, errListSpaces)
	}

	if len(l.Items) == 0 {
		printer.Println("No spaces found")
		return nil
	}

	fieldNames := []string{"NAME", "MODE", "PROVIDER", "REGION"}
	return printer.PrintObject(l.Items, fieldNames, extractSpaceListFields)
}

func extractSpaceListFields(obj any) []string {
	space, ok := obj.(upboundv1alpha1.Space)
	if !ok {
		return []string{"unknown", "unknown", "", ""}
	}

	provider, region := "", ""
	if space.Spec.Provider != nil {
		provider = string(*space.Spec.Provider)
	}

	if space.Spec.Region != nil {
		region = string(*space.Spec.Region)
	}

	mode := space.Labels[upboundv1alpha1.SpaceModeLabelKey]

	return []string{
		space.GetObjectMeta().GetName(),
		mode,
		provider,
		region,
	}
}
