// Copyright 2025 Upbound Inc.
// All rights reserved

// Package config provides commands for managing configuration.
package config

import (
	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/feature"

	_ "embed"
)

const (
	errFailedToReadConfig  = "failed to read config"
	errFailedToWriteConfig = "failed to write config"
)

// BeforeReset is the first hook to run.
func (c *Cmd) BeforeReset(p *kong.Path, maturity feature.Maturity) error {
	return feature.HideMaturity(p, maturity)
}

// Cmd contains commands for managing configuration.
type Cmd struct {
	Set setCmd `cmd:"" help:"Set configuration values."`
	Get getCmd `cmd:"" help:"Get configuration values."`
}

// setCmd sets configuration values.
type setCmd struct {
	Key   string `arg:"" help:"Configuration key to set."`
	Value string `arg:"" help:"Configuration value to set."`
}

type getCmd struct{}

func (c *getCmd) Run(p pterm.TextPrinter) error {
	src := config.NewFSSource()
	if err := src.Initialize(); err != nil {
		return errors.Wrap(err, "failed to initialize config")
	}

	conf, err := config.Extract(src)
	if err != nil {
		return errors.Wrap(err, errFailedToReadConfig)
	}

	values := conf.GetBaseConfiguration()

	p.Printfln("Configuration:")
	for key, value := range values {
		p.Printfln("- %s = %s", key, value)
	}

	return nil
}

//go:embed help/get.md
var getHelp string

func (c *getCmd) Help() string {
	return getHelp
}

//go:embed help/set.md
var setHelp string

func (c *setCmd) Help() string {
	return setHelp
}

// Run sets a configuration value.
func (c *setCmd) Run(p pterm.TextPrinter) error {
	// Get config source
	src := config.NewFSSource()
	if err := src.Initialize(); err != nil {
		return errors.Wrap(err, "failed to initialize config")
	}

	conf, err := config.Extract(src)
	if err != nil {
		return errors.Wrap(err, errFailedToReadConfig)
	}

	values := conf.GetBaseConfiguration()
	if !config.IsConfigurationFlag(c.Key) {
		p.Printfln("Invalid configuration key: %s", c.Key)
		p.Printfln("Valid configuration keys:")
		flags, err := config.GetValidUserExposedConfigurationFlags()
		if err != nil {
			return errors.Wrap(err, "failed to get valid configuration flags")
		}
		for key, flag := range flags {
			p.Printfln("- %s: %s", key, flag.Description)
		}
		return nil
	}

	if v, ok := values[c.Key]; ok && v == c.Value {
		p.Printfln("Configuration already set: %s = %s", c.Key, c.Value)
		return nil
	}

	conf.SetBaseConfiguration(c.Key, c.Value)

	if err := src.UpdateConfig(conf); err != nil {
		return errors.Wrap(err, errFailedToWriteConfig)
	}

	p.Printfln("Configuration updated: %s = %s", c.Key, c.Value)
	return nil
}
