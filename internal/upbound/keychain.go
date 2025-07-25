// Copyright 2025 Upbound Inc.
// All rights reserved

package upbound

import (
	"github.com/google/go-containerregistry/pkg/authn"

	"github.com/upbound/up/internal/credhelper"
	"github.com/upbound/up/internal/profile"
)

// RegistryKeychain returns a container registry keychain that can access the
// context's organization's repositories as well as any registries in the user's
// default keychain (e.g., docker logins).
func (c *Context) RegistryKeychain() authn.Keychain {
	// robot tokens cannot be used for registry login 401 Unauthorized
	if c.Profile.TokenType == profile.TokenTypeRobot {
		return authn.DefaultKeychain
	}

	credHelperKeychain := authn.NewKeychainFromHelper(
		credhelper.New(
			credhelper.WithDomain(c.RegistryEndpoint.Host),
			credhelper.WithProfile(c.ProfileName),
		),
	)

	return authn.NewMultiKeychain(credHelperKeychain, authn.DefaultKeychain)
}
