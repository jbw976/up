// Copyright 2024 Upbound Inc
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

// Package git contains functions to interact with repos
package git

import (
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-git/go-git/v5/storage"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// CloneOptions configure for git actions.
type CloneOptions struct {
	Repo      string
	RefName   string
	Directory string
	Method    string
}

// AuthProvider wraps a specific auth method.
type AuthProvider interface {
	GetAuthMethod() (transport.AuthMethod, error)
}

// HTTPSAuthProvider provides authentication for HTTPS repositories.
type HTTPSAuthProvider struct {
	Username string
	Password string
}

// GetAuthMethod returns the HTTP BasicAuth transport method.
func (a *HTTPSAuthProvider) GetAuthMethod() (transport.AuthMethod, error) {
	if a.Username != "" || a.Password != "" {
		return &http.BasicAuth{Username: a.Username, Password: a.Password}, nil
	}
	// Return nil authenticator to allow anonymous cloning.
	return nil, nil
}

// SSHAuthProvider provides authentication for SSH repositories.
type SSHAuthProvider struct {
	Username       string
	PrivateKeyPath string
	Passphrase     string
}

// GetAuthMethod returns the SSH PublicKey transport method.
func (a *SSHAuthProvider) GetAuthMethod() (transport.AuthMethod, error) {
	return ssh.NewPublicKeysFromFile(a.Username, a.PrivateKeyPath, a.Passphrase)
}

// Cloner can clone git repositories with (optional) authentication.
type Cloner interface {
	CloneRepository(store storage.Storer, fs billy.Filesystem, auth AuthProvider, opts CloneOptions) (*plumbing.Reference, error)
}

// DefaultCloner is the default implementation of Cloner.
type DefaultCloner struct{}

// CloneRepository clones a git repository using the provided CloneOptions and AuthProvider.
func (dc *DefaultCloner) CloneRepository(store storage.Storer, fs billy.Filesystem, auth AuthProvider, opts CloneOptions) (*plumbing.Reference, error) {
	// Get the authentication method from the AuthProvider.
	authMethod, err := auth.GetAuthMethod()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get authentication method")
	}

	cloneOptions := &git.CloneOptions{
		URL:           opts.Repo,
		Depth:         1,
		ReferenceName: plumbing.ReferenceName(opts.RefName),
		Auth:          authMethod,
	}

	// Clone the repository.
	repoObj, err := git.Clone(store, fs, cloneOptions)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to clone repository from %q", opts.Repo)
	}

	// Get the HEAD reference.
	ref, err := repoObj.Head()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get repository's HEAD from %q", opts.Repo)
	}
	return ref, nil
}
