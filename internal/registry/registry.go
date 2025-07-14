// Copyright 2025 Upbound Inc.
// All rights reserved

// Package registry contains types for OCI registries.
package registry

import (
	"net/url"
	"os"
	"strings"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/input"
	"github.com/upbound/up/internal/upbound"
)

const (
	upboundRegistry = "xpkg.upbound.io"
)

// Flags contains flags for specifying an OCI registry.
type Flags struct {
	Repository url.URL `default:"xpkg.upbound.io/spaces-artifacts" env:"UPBOUND_REGISTRY"          help:"Set registry for where to pull OCI artifacts from. This is an OCI registry reference, i.e. a URL without the scheme or protocol prefix." hidden:"" name:"registry-repository"`
	Endpoint   url.URL `default:"https://xpkg.upbound.io"          env:"UPBOUND_REGISTRY_ENDPOINT" help:"Set registry endpoint, including scheme, for authentication."                                                                            hidden:"" name:"registry-endpoint"`
}

// AuthorizedFlags contains flags for specifying an OCI registry and credentials
// to authenticate with it.
type AuthorizedFlags struct {
	Flags

	TokenFile *os.File `help:"File containing authentication token. Expecting a JSON file with \"accessId\" and \"token\" keys." name:"token-file"`
	Username  string   `help:"Set the registry username."                                                                        hidden:""         name:"registry-username"`
	Password  string   `help:"Set the registry password."                                                                        hidden:""         name:"registry-password"`
}

// AfterApply sets default values in AuthorizedFlags after assignment and
// validation.
func (f *AuthorizedFlags) AfterApply() error {
	if f.TokenFile == nil && f.Username == "" && f.Password == "" {
		if strings.HasPrefix(f.Repository.String(), upboundRegistry) {
			return errors.New("--token-file is required")
		}

		prompter := input.NewPrompter()
		id, err := prompter.Prompt("Username", false)
		if err != nil {
			return err
		}
		token, err := prompter.Prompt("Password", true)
		if err != nil {
			return err
		}
		f.Username = id
		f.Password = token

		return nil
	}

	if f.Username != "" {
		return nil
	}

	tf, err := upbound.TokenFromPath(f.TokenFile.Name())
	if err != nil {
		return err
	}
	f.Username, f.Password = tf.AccessID, tf.Token

	return nil
}
