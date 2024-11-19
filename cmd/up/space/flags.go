// Copyright 2023 Upbound Inc.
// All rights reserved

package space

import (
	"net/url"
	"os"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/input"
	"github.com/upbound/up/internal/upbound"
)

type registryFlags struct {
	Repository *url.URL `default:"xpkg.upbound.io/spaces-artifacts" env:"UPBOUND_REGISTRY"          help:"Set registry for where to pull OCI artifacts from. This is an OCI registry reference, i.e. a URL without the scheme or protocol prefix." hidden:"" name:"registry-repository"`
	Endpoint   *url.URL `default:"https://xpkg.upbound.io"          env:"UPBOUND_REGISTRY_ENDPOINT" help:"Set registry endpoint, including scheme, for authentication."                                                                            hidden:"" name:"registry-endpoint"`
}

type authorizedRegistryFlags struct {
	registryFlags

	TokenFile *os.File `help:"File containing authentication token. Expecting a JSON file with \"accessId\" and \"token\" keys." name:"token-file"`
	Username  string   `help:"Set the registry username."                                                                        hidden:""         name:"registry-username"`
	Password  string   `help:"Set the registry password."                                                                        hidden:""         name:"registry-password"`
}

func (p *authorizedRegistryFlags) AfterApply() error {
	if p.TokenFile == nil && p.Username == "" && p.Password == "" {
		if p.Repository.String() == defaultRegistry {
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
		p.Username = id
		p.Password = token

		return nil
	}

	if p.Username != "" {
		return nil
	}

	tf, err := upbound.TokenFromPath(p.TokenFile.Name())
	if err != nil {
		return err
	}
	p.Username, p.Password = tf.AccessID, tf.Token

	return nil
}
