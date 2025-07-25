// Copyright 2025 Upbound Inc.
// All rights reserved

// Package profile contains types for up CLI configuration profiles.
package profile

import (
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/yaml"
)

// TokenType is a type of Upbound session token format.
type TokenType string

const (
	// TokenTypeUser is the user type of token.
	TokenTypeUser TokenType = "user"
	// TokenTypeRobot is the robot type of token.
	TokenTypeRobot TokenType = "robot"
	// TokenTypePAT is the token type of token.
	TokenTypePAT TokenType = "token"
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

	RawSpaceKubeconfig json.RawMessage `json:"spaceKubeconfig,omitempty"`

	// SpaceKubeconfig is the kubeconfig for the disconnected space in a
	// disconnected profile. It must not be set for cloud profiles.
	SpaceKubeconfig *clientcmdapi.Config `json:"-"`

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

	if p.RawSpaceKubeconfig != nil {
		p.SpaceKubeconfig = new(clientcmdapi.Config)

		var meta metav1.TypeMeta
		if err := yaml.Unmarshal(p.RawSpaceKubeconfig, &meta); err != nil {
			return err
		}
		if meta.APIVersion == "" && meta.Kind == "" {
			// Config was written by an old version of up, which serialized the
			// clientcmdapi.Config directly instead of converting it to a
			// versioned one first.
			if err := json.Unmarshal(p.RawSpaceKubeconfig, p.SpaceKubeconfig); err != nil {
				return err
			}
		} else {
			// Config was written by a newer version of up, which serialized a
			// versioned config.
			kc, err := clientcmd.Load(p.RawSpaceKubeconfig)
			if err != nil {
				return err
			}
			p.SpaceKubeconfig = kc
		}
	}

	return nil
}

// MarshalJSON marshals a profile to JSON, converting the SpaceKubeconfig to a
// RawSpaceKubeconfig appropriate for storage.
func (p Profile) MarshalJSON() ([]byte, error) {
	type profile Profile

	if p.SpaceKubeconfig != nil {
		bs, err := clientcmd.Write(*p.SpaceKubeconfig)
		if err != nil {
			return nil, err
		}
		p.RawSpaceKubeconfig, err = yaml.YAMLToJSON(bs)
		if err != nil {
			return nil, err
		}
	}

	pc := profile(p)
	return json.Marshal(pc)
}

// Validate returns an error if the profile is invalid.
func (p *Profile) Validate() error {
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
		bs, err := yaml.Marshal(pc.SpaceKubeconfig)
		if err != nil {
			return nil, err
		}
		p.RawSpaceKubeconfig, err = yaml.YAMLToJSON(bs)
		if err != nil {
			return nil, err
		}
	}
	return json.Marshal(&pc)
}
