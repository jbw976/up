// Copyright 2025 Upbound Inc.
// All rights reserved

package profile

import (
	"encoding/json"
	"testing"

	"gotest.tools/v3/assert"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestValidate(t *testing.T) {
	tcs := map[string]struct {
		input   Profile
		errText string
	}{
		"ValidDisconnectedNoOrganization": {
			input: Profile{
				Type:            TypeDisconnected,
				SpaceKubeconfig: &clientcmdapi.Config{},
			},
		},
		"ValidDisconnectedWithOrganization": {
			input: Profile{
				Type:            TypeDisconnected,
				TokenType:       TokenTypeUser,
				Organization:    "my-org",
				Session:         "session-token",
				Domain:          "https://upbound.io",
				SpaceKubeconfig: &clientcmdapi.Config{},
			},
		},
		"ValidCloud": {
			input: Profile{
				Type:         TypeCloud,
				TokenType:    TokenTypeUser,
				Organization: "my-org",
				Session:      "session-token",
				Domain:       "https://upbound.io",
			},
		},
		"InvalidCloudWithKubeConfig": {
			input: Profile{
				Type:            TypeCloud,
				TokenType:       TokenTypeUser,
				Organization:    "my-org",
				Session:         "session-token",
				Domain:          "https://upbound.io",
				SpaceKubeconfig: &clientcmdapi.Config{},
			},
			errText: "kubeconfig must not be set for cloud profiles",
		},
		"InvalidCloudMissingOrganization": {
			input: Profile{
				Type:      TypeCloud,
				TokenType: TokenTypeUser,
				Session:   "session-token",
				Domain:    "https://upbound.io",
			},
			errText: "organization must be set for cloud profiles",
		},
		"InvalidDisconnectedMissingKubeConfig": {
			input: Profile{
				Type:      TypeDisconnected,
				TokenType: TokenTypeUser,
				Session:   "session-token",
				Domain:    "https://upbound.io",
			},
			errText: "kubeconfig must be set for disconnected profiles",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			err := tc.input.Validate()
			if tc.errText == "" {
				assert.NilError(t, err)
			} else {
				assert.Error(t, err, tc.errText)
			}
		})
	}
}

func TestRedact(t *testing.T) {
	p := Profile{
		Type:         TypeDisconnected,
		TokenType:    TokenTypeUser,
		Session:      "secret-session-token",
		Organization: "my-org",
		Domain:       "https://upbound.io",
		SpaceKubeconfig: &clientcmdapi.Config{
			CurrentContext: "default",
			Clusters: map[string]*clientcmdapi.Cluster{
				"default": {},
			},
			AuthInfos: map[string]*clientcmdapi.AuthInfo{
				"default": {
					ClientCertificateData: []byte("my-certificate-bytes"),
					ClientKeyData:         []byte("secret-key-bytes"),
					Token:                 "secret-token",
					Username:              "my-username",
					Password:              "secret-password",
				},
			},
			Contexts: map[string]*clientcmdapi.Context{
				"default": {
					AuthInfo:  "default",
					Cluster:   "default",
					Namespace: "default",
				},
			},
		},
	}

	r := Redacted{p}

	bs, err := r.MarshalJSON()
	assert.NilError(t, err)

	var roundTrip Profile
	err = json.Unmarshal(bs, &roundTrip)
	assert.NilError(t, err)

	assert.DeepEqual(t, roundTrip, Profile{
		Type:         TypeDisconnected,
		TokenType:    TokenTypeUser,
		Session:      "REDACTED",
		Organization: "my-org",
		Domain:       "https://upbound.io",
		SpaceKubeconfig: &clientcmdapi.Config{
			CurrentContext: "default",
			Clusters: map[string]*clientcmdapi.Cluster{
				"default": {},
			},
			AuthInfos: map[string]*clientcmdapi.AuthInfo{
				"default": {
					ClientCertificateData: []byte("my-certificate-bytes"),
					ClientKeyData:         []byte("REDACTED"),
					Token:                 "REDACTED",
					Username:              "my-username",
					Password:              "REDACTED",
				},
			},
			Contexts: map[string]*clientcmdapi.Context{
				"default": {
					AuthInfo:  "default",
					Cluster:   "default",
					Namespace: "default",
				},
			},
		},
	})
}
