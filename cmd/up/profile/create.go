// Copyright 2025 Upbound Inc.
// All rights reserved

package profile

import (
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/profile"
	"github.com/upbound/up/internal/upbound"
)

type createCmd struct {
	Name string `arg:""                                                                             help:"Name of the profile to create." required:""`
	Use  bool   `help:"Use the new profile after it's created. Default if no other profile exists."`

	Type profile.Type `default:"cloud" enum:"cloud,disconnected" help:"Type of profile to create: cloud or disconnected."`
}

func (c *createCmd) AfterApply(flags upbound.Flags, upCtx *upbound.Context) error {
	if c.Type == profile.TypeCloud && flags.Organization == "" {
		return errors.New("organization is required for cloud profiles")
	}

	if _, err := upCtx.Cfg.GetUpboundProfile(c.Name); err == nil {
		return errors.Errorf("a profile named %q already exists; use `up profile set` to update it if desired", c.Name)
	}

	return nil
}

func (c *createCmd) Run(flags upbound.Flags, upCtx *upbound.Context) error {
	p := profile.Profile{
		Type:         c.Type,
		Organization: flags.Organization,
		Domain:       upCtx.Domain.String(),
	}

	if p.Type == profile.TypeDisconnected {
		kc, err := upCtx.GetRawKubeconfig()
		if err != nil {
			return errors.Wrap(err, "failed to get kubeconfig")
		}

		if err := clientcmdapi.MinifyConfig(&kc); err != nil {
			return errors.Wrap(err, "failed to create kubeconfig for disconnected profile")
		}

		p.SpaceKubeconfig = &kc
	}

	if err := upCtx.Cfg.AddOrUpdateUpboundProfile(c.Name, p); err != nil {
		return err
	}

	if c.Use || len(upCtx.Cfg.Upbound.Profiles) == 1 {
		if err := upCtx.Cfg.SetDefaultUpboundProfile(c.Name); err != nil {
			return errors.Wrap(err, "failed to use new profile")
		}
	}

	return errors.Wrap(upCtx.CfgSrc.UpdateConfig(upCtx.Cfg), "unable to create profile")
}
