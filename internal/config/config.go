// Copyright 2025 Upbound Inc.
// All rights reserved

// Package config handles the up CLI configuration file and types.
package config

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/profile"
)

// Location of up config file.
const (
	ConfigDir  = ".up"
	ConfigFile = "config.json"
)

const (
	errDefaultNotExist    = "profile specified as default does not exist"
	errNoDefaultSpecified = "no default profile specified"

	errProfileNotFoundFmt      = "profile not found with identifier: %s"
	errProfileAlreadyExistsFmt = "profile already exists with identifier: %s"
	errNoProfilesFound         = "no profiles found"
)

const (
	// DefaultDomain is the default Upbound domain used for constructing API
	// endpoints.
	DefaultDomain = "https://upbound.io"
)

// QuietFlag provides a named boolean type for the QuietFlag.
type QuietFlag bool

// Format represents allowed values for the global output format option.
type Format string

const (
	// FormatDefault is the default, human-friendly, output format.
	FormatDefault Format = "default"
	// FormatJSON is the JSON output format.
	FormatJSON Format = "json"
	// FormatYAML is the YAML output format.
	FormatYAML Format = "yaml"
)

// Config is format for the up configuration file.
type Config struct {
	Upbound Upbound `json:"upbound"`
}

// Extract performs extraction of configuration from the provided source.
func Extract(src Source) (*Config, error) {
	conf, err := src.GetConfig()
	if err != nil {
		return nil, err
	}
	return conf, nil
}

// GetDefaultPath returns the default config path or error.
func GetDefaultPath() (string, error) {
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ConfigDir, ConfigFile), nil
}

// Upbound contains configuration information for Upbound.
type Upbound struct {
	// Default indicates the default profile.
	Default string `json:"default"`

	// Profiles contain sets of credentials for communicating with Upbound. Key
	// is name of the profile.
	Profiles map[string]profile.Profile `json:"profiles,omitempty"`
}

// AddOrUpdateUpboundProfile adds or updates an Upbound profile to the Config.
func (c *Config) AddOrUpdateUpboundProfile(name string, p profile.Profile) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if c.Upbound.Profiles == nil {
		c.Upbound.Profiles = map[string]profile.Profile{}
	}
	c.Upbound.Profiles[name] = p
	return nil
}

// DeleteUpboundProfile deletes an Upbound profile from the Config. If it is the
// current profile, an arbitrary remaining profile will be chosen as the new
// default.
func (c *Config) DeleteUpboundProfile(name string) error {
	if c.Upbound.Profiles == nil {
		return errors.Errorf(errProfileNotFoundFmt, name)
	}
	if _, ok := c.Upbound.Profiles[name]; !ok {
		return errors.Errorf(errProfileNotFoundFmt, name)
	}

	delete(c.Upbound.Profiles, name)
	// If the deleted profile was the default, set the default to an arbitrary
	// profile, or empty it if there are no profiles remaining.
	if c.Upbound.Default == name {
		c.Upbound.Default = ""
		for k := range c.Upbound.Profiles {
			c.Upbound.Default = k
			break
		}
	}

	return nil
}

// RenameUpboundProfile renames an Upbound profile in the Config. If it is the
// current profile the default will be updated to match. If a profile with the
// new name already exists an error will be returned.
func (c *Config) RenameUpboundProfile(from, to string) error {
	if c.Upbound.Profiles == nil {
		return errors.Errorf(errProfileNotFoundFmt, from)
	}
	p, ok := c.Upbound.Profiles[from]
	if !ok {
		return errors.Errorf(errProfileNotFoundFmt, from)
	}
	if from == to {
		return nil
	}
	if _, ok := c.Upbound.Profiles[to]; ok {
		return errors.Errorf(errProfileAlreadyExistsFmt, to)
	}

	c.Upbound.Profiles[to] = p
	delete(c.Upbound.Profiles, from)

	if c.Upbound.Default == from {
		c.Upbound.Default = to
	}

	return nil
}

// GetDefaultUpboundProfile gets the default Upbound profile or returns an error if
// default is not set or default profile does not exist.
func (c *Config) GetDefaultUpboundProfile() (string, profile.Profile, error) {
	if c.Upbound.Default == "" {
		return "", profile.Profile{}, errors.New(errNoDefaultSpecified)
	}
	p, ok := c.Upbound.Profiles[c.Upbound.Default]
	if !ok {
		return "", profile.Profile{}, errors.New(errDefaultNotExist)
	}
	return c.Upbound.Default, p, nil
}

// GetUpboundProfile gets a profile with a given identifier. If a profile does not
// exist for the given identifier an error will be returned. Multiple profiles
// should never exist for the same identifier, but in the case that they do, the
// first will be returned.
func (c *Config) GetUpboundProfile(name string) (profile.Profile, error) {
	p, ok := c.Upbound.Profiles[name]
	if !ok {
		return profile.Profile{}, errors.Errorf(errProfileNotFoundFmt, name)
	}
	return p, nil
}

// GetUpboundProfiles returns the list of existing profiles. If no profiles
// exist, then an error will be returned.
func (c *Config) GetUpboundProfiles() (map[string]profile.Profile, error) {
	if c.Upbound.Profiles == nil {
		return nil, errors.New(errNoProfilesFound)
	}

	return c.Upbound.Profiles, nil
}

// SetDefaultUpboundProfile sets the default profile for communicating with
// Upbound. Setting a default profile that does not exist will return an
// error.
func (c *Config) SetDefaultUpboundProfile(name string) error {
	if _, ok := c.Upbound.Profiles[name]; !ok {
		return errors.Errorf(errProfileNotFoundFmt, name)
	}
	c.Upbound.Default = name
	return nil
}

// GetBaseConfig returns the persisted base configuration associated with the
// provided Profile. If the supplied name does not match an existing Profile
// an error is returned.
func (c *Config) GetBaseConfig(name string) (map[string]string, error) {
	profile, ok := c.Upbound.Profiles[name]
	if !ok {
		return nil, errors.Errorf(errProfileNotFoundFmt, name)
	}
	return profile.BaseConfig, nil
}

// AddToBaseConfig adds the supplied key, value pair to the base config map of
// the profile that corresponds to the given name. If the supplied name does
// not match an existing Profile an error is returned. If the overrides map
// does not currently exist on the corresponding profile, a map is initialized.
func (c *Config) AddToBaseConfig(name, key, value string) error {
	profile, ok := c.Upbound.Profiles[name]
	if !ok {
		return errors.Errorf(errProfileNotFoundFmt, name)
	}

	if profile.BaseConfig == nil {
		profile.BaseConfig = make(map[string]string)
	}

	profile.BaseConfig[key] = value
	c.Upbound.Profiles[name] = profile
	return nil
}

// RemoveFromBaseConfig removes the supplied key from the base config map of
// the Profile that corresponds to the given name. If the supplied name does
// not match an existing Profile an error is returned. If the base config map
// does not currently exist on the corresponding profile, a no-op occurs.
func (c *Config) RemoveFromBaseConfig(name, key string) error {
	profile, ok := c.Upbound.Profiles[name]
	if !ok {
		return errors.Errorf(errProfileNotFoundFmt, name)
	}

	if profile.BaseConfig == nil {
		return nil
	}

	delete(profile.BaseConfig, key)
	c.Upbound.Profiles[name] = profile
	return nil
}

// BaseToJSON converts the base config of the given Profile to JSON. If the
// config couldn't be converted or if the supplied name does not correspond
// to an existing Profile, an error is returned.
func (c *Config) BaseToJSON(name string) (io.Reader, error) {
	profile, err := c.GetBaseConfig(name)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(profile); err != nil {
		return nil, err
	}

	return &buf, nil
}

func (c *Config) applyDefaults() {
	for _, p := range c.Upbound.Profiles {
		if p.Domain == "" {
			p.Domain = DefaultDomain
		}
	}
}
