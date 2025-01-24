// Copyright 2025 Upbound Inc.
// All rights reserved

package config

import (
	"os"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/upbound"
)

type unsetCmd struct {
	Key string `arg:"" help:"Configuration Key." optional:""`

	File *os.File `help:"Configuration File. Must be in JSON format." short:"f"`
}

func (c *unsetCmd) Run(upCtx *upbound.Context) error {
	if err := c.validateInput(); err != nil {
		return err
	}

	profile, _, err := upCtx.Cfg.GetDefaultUpboundProfile()
	if err != nil {
		return err
	}

	cfg := map[string]any{
		c.Key: 0,
	}
	if c.File != nil {
		cfg, err = mapFromFile(c.File)
		if err != nil {
			return err
		}
	}

	if err := c.removeConfigs(upCtx, profile, cfg); err != nil {
		return err
	}
	return errors.Wrap(upCtx.CfgSrc.UpdateConfig(upCtx.Cfg), errUpdateConfig)
}

func (c *unsetCmd) validateInput() error {
	if c.Key != "" && c.File == nil {
		return nil
	}
	if c.Key == "" && c.File != nil {
		return nil
	}

	return errors.New(errOnlyKVFileXOR)
}

func (c *unsetCmd) removeConfigs(upCtx *upbound.Context, profile string, config map[string]any) error {
	for k := range config {
		if err := upCtx.Cfg.RemoveFromBaseConfig(profile, k); err != nil {
			return err
		}
	}
	return nil
}
