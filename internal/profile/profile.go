// Copyright 2023 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package profile contains types for up CLI configuration profiles.
package profile

import (
	"encoding/json"
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

// A Profile is a set of credentials.
type Profile struct {
	// ID is the referencable name of the profile.
	ID string `json:"id,omitempty"`

	// TokenType is the type of token in the profile.
	TokenType TokenType `json:"type"`

	// Session is a session token used to authenticate to Upbound.
	Session string `json:"session,omitempty"`

	// Account is the default account to use when this profile is selected.
	Account string `json:"account,omitempty"`

	// Domain is the base domain used to construct URLs when this profile is
	// selected.
	Domain string `json:"domain,omitempty"`

	// BaseConfig represent persisted settings for this profile.
	// For example:
	// * flags
	// * environment variables
	BaseConfig map[string]string `json:"base,omitempty"`
}

// Validate returns an error if the profile is invalid.
func (p Profile) Validate() error {
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
	return json.Marshal(&pc)
}
