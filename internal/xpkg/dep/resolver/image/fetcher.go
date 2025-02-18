// Copyright 2025 Upbound Inc.
// All rights reserved

// Package image contains a dependency resolver that works with OCI images.
package image

import (
	"context"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// LocalFetcher --.
type LocalFetcher struct {
	keychain authn.Keychain
}

// LocalFetcherOption modifies the fetcher.
type LocalFetcherOption func(*LocalFetcher)

// WithKeychain sets the auth keychain the fetcher will use.
func WithKeychain(kc authn.Keychain) LocalFetcherOption {
	return func(f *LocalFetcher) {
		f.keychain = kc
	}
}

// NewLocalFetcher --.
func NewLocalFetcher(opts ...LocalFetcherOption) *LocalFetcher {
	f := &LocalFetcher{
		keychain: authn.DefaultKeychain,
	}

	for _, opt := range opts {
		opt(f)
	}

	return f
}

// Fetch fetches a package image.
func (r *LocalFetcher) Fetch(ctx context.Context, ref name.Reference, _ ...string) (v1.Image, error) {
	return remote.Image(ref, remote.WithContext(ctx), remote.WithAuthFromKeychain(r.keychain))
}

// Head fetches a package descriptor.
func (r *LocalFetcher) Head(ctx context.Context, ref name.Reference, _ ...string) (*v1.Descriptor, error) {
	return remote.Head(ref, remote.WithContext(ctx), remote.WithAuthFromKeychain(r.keychain))
}

// Tags fetches a package's tags.
func (r *LocalFetcher) Tags(ctx context.Context, ref name.Reference, _ ...string) ([]string, error) {
	return remote.List(ref.Context(), remote.WithContext(ctx), remote.WithAuthFromKeychain(r.keychain))
}
