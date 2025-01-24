// Copyright 2025 Upbound Inc.
// All rights reserved

package image

import (
	"context"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// LocalFetcher --.
type LocalFetcher struct{}

// NewLocalFetcher --.
func NewLocalFetcher() *LocalFetcher {
	return &LocalFetcher{}
}

// Fetch fetches a package image.
func (r *LocalFetcher) Fetch(ctx context.Context, ref name.Reference, secrets ...string) (v1.Image, error) {
	return remote.Image(ref, remote.WithContext(ctx), remote.WithAuthFromKeychain(authn.DefaultKeychain))
}

// Head fetches a package descriptor.
func (r *LocalFetcher) Head(ctx context.Context, ref name.Reference, secrets ...string) (*v1.Descriptor, error) {
	return remote.Head(ref, remote.WithContext(ctx), remote.WithAuthFromKeychain(authn.DefaultKeychain))
}

// Tags fetches a package's tags.
func (r *LocalFetcher) Tags(ctx context.Context, ref name.Reference, secrets ...string) ([]string, error) {
	return remote.List(ref.Context(), remote.WithContext(ctx), remote.WithAuthFromKeychain(authn.DefaultKeychain))
}
