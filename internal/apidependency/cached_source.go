// Copyright 2025 Upbound Inc.
// All rights reserved

package apidependency

import (
	"context"

	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/schemas/manager"
	"github.com/upbound/up/pkg/apis/project/v1alpha1"
)

// CachedSource wraps a manager.Source with caching capabilities.
type CachedSource struct {
	source manager.Source
	cache  Cache
	dep    v1alpha1.APIDependencies
}

// NewCachedSource creates a new cached source wrapper.
func NewCachedSource(source manager.Source, cache Cache, dep v1alpha1.APIDependencies) manager.Source {
	return &CachedSource{
		source: source,
		cache:  cache,
		dep:    dep,
	}
}

// ID returns the source ID.
func (c *CachedSource) ID() string {
	return c.source.ID()
}

// Version returns the source version.
func (c *CachedSource) Version(ctx context.Context) (string, error) {
	// Always delegate to the underlying source for version
	// The cache doesn't store version information separately
	return c.source.Version(ctx)
}

// Resources returns the resources, using cache when possible.
func (c *CachedSource) Resources(ctx context.Context) (afero.Fs, error) {
	// Check cache first
	if fs, err := c.cache.Get(c.dep); err == nil {
		// Cache hit - return the cached filesystem
		return fs, nil
	}

	// Cache miss - fetch from source
	fs, err := c.source.Resources(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch resources from source")
	}

	// Store in cache for future use
	// Cache store failures are non-critical since we have the data
	_ = c.cache.Store(c.dep, fs)

	return fs, nil
}

// Type returns the source type.
func (c *CachedSource) Type() manager.SourceType {
	return c.source.Type()
}
