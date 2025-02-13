// Copyright 2025 Upbound Inc.
// All rights reserved

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
	// Use default username if none is provided
	username := a.Username
	if username == "" {
		username = "git"
	}

	// Attempt to create public key auth method
	authMethod, err := ssh.NewPublicKeysFromFile(username, a.PrivateKeyPath, a.Passphrase)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create SSH public key auth method for user %q", username)
	}

	return authMethod, nil
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
