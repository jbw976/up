// Copyright 2025 Upbound Inc.
// All rights reserved

// Package profile contains types for up CLI configuration profiles.
package profile

import (
	"encoding/json"

	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// TokenType is a type of Upbound session token format.
type TokenType string

const (
	// TokenTypeUser is the user type of token.
	TokenTypeUser TokenType = "user"
	// TokenTypeToken is the token type of token.
	TokenTypeToken TokenType = "token"

	// DefaultName is the default profile name.
	DefaultName = "default"
)

// Type distinguishes cloud profiles from disconnected profiles. Cloud profiles
// are used to interact with Upbound cloud and connected Spaces, while
// disconnected profiles interact with self-hosted, disconnected Spaces.
type Type string

const (
	// TypeCloud is the profile type for cloud profiles.
	TypeCloud Type = "cloud"
	// TypeDisconnected is the profile type for disconnected profiles.
	TypeDisconnected Type = "disconnected"
)

// A Profile is a set of credentials.
type Profile struct {
	// ID is the referencable name of the profile.
	ID string `json:"id,omitempty"`

	// Type is the type of profile. If empty, cloud is assumed.
	Type Type `json:"profileType,omitempty"`

	// TokenType is the type of token in the profile.
	TokenType TokenType `json:"type"`

	// Session is a session token used to authenticate to Upbound.
	Session string `json:"session,omitempty"`

	// Account is the default account to use when this profile is selected.
	//
	// Deprecated: Use Organization instead.
	Account string `json:"account,omitempty"`
	// Organization is the organization associated with this profile.
	Organization string `json:"organization,omitempty"`

	// Domain is the base domain used to construct URLs when this profile is
	// selected.
	Domain string `json:"domain,omitempty"`

	// SpaceKubeconfig is the kubeconfig for the disconnected space in a
	// disconnected profile. It must not be set for cloud profiles.
	SpaceKubeconfig *clientcmdapi.Config `json:"spaceKubeconfig,omitempty"`

	// CurrentKubeContext is the current kubeconfig context for the profile, in
	// the format used by non-interactive `up ctx`.
	CurrentKubeContext string `json:"currentKubeContext,omitempty"`

	// BaseConfig represent persisted settings for this profile.
	// For example:
	// * flags
	// * environment variables
	BaseConfig map[string]string `json:"base,omitempty"`
}

// UnmarshalJSON unmarshals the JSON representation of a profile, handling field
// upgrades and defaults.
func (p *Profile) UnmarshalJSON(bs []byte) error {
	type profile Profile
	var pc profile
	if err := json.Unmarshal(bs, &pc); err != nil {
		return err
	}

	*p = Profile(pc)

	if p.Organization == "" {
		p.Organization = p.Account
	}
	if p.Type == "" {
		p.Type = TypeCloud
	}

	return nil
}

// Validate returns an error if the profile is invalid.
func (p Profile) Validate() error {
	switch p.Type {
	case TypeDisconnected:
		if p.SpaceKubeconfig == nil {
			return errors.New("kubeconfig must be set for disconnected profiles")
		}

	case TypeCloud:
		if p.SpaceKubeconfig != nil {
			return errors.New("kubeconfig must not be set for cloud profiles")
		}
		if p.Organization == "" {
			return errors.New("organization must be set for cloud profiles")
		}
	}

	return nil
}

// Redacted embeds a Upbound Profile for the sole purpose of redacting
// sensitive information.
type Redacted struct {
	Profile
}

// MarshalJSON overrides the session field with `REDACTED` so as not to leak
// sensitive information. We're using an explicit copy here instead of updating
// the underlying Profile struct so as to not modifying the internal state of
// the struct by accident.
func (p Redacted) MarshalJSON() ([]byte, error) {
	type profile Redacted
	pc := profile(p)
	s := "NONE"
	if pc.Session != "" {
		s = "REDACTED"
	}
	pc.Session = s
	if pc.SpaceKubeconfig != nil {
		if err := clientcmdapi.RedactSecrets(pc.SpaceKubeconfig); err != nil {
			return nil, errors.Wrap(err, "failed to redact kubeconfig")
		}
	}
	return json.Marshal(&pc)
}
