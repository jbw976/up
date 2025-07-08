// Copyright 2025 Upbound Inc.
// All rights reserved

package apidependency

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/pkg/apis/project/v1alpha1"
)

const (
	// cacheVersion is incremented when the cache format changes
	// to ensure old caches are invalidated.
	cacheVersion = "v1"
)

// Cache provides caching capabilities for API dependencies.
type Cache interface {
	// Get retrieves a cached API dependency filesystem
	Get(dep v1alpha1.APIDependencies) (afero.Fs, error)

	// Store saves an API dependency filesystem to the cache.
	Store(dep v1alpha1.APIDependencies, fs afero.Fs) error
}

// LocalCache implements a filesystem-based cache for API dependencies.
type LocalCache struct {
	root string
	log  logging.Logger
	mu   sync.RWMutex
	fs   afero.Fs
}

// LocalCacheOption configures a LocalCache.
type LocalCacheOption func(*LocalCache)

// WithLogger sets the logger for the cache.
func WithLogger(log logging.Logger) LocalCacheOption {
	return func(c *LocalCache) {
		c.log = log
	}
}

// WithFS sets the filesystem for the cache.
func WithFS(fs afero.Fs) LocalCacheOption {
	return func(c *LocalCache) {
		c.fs = fs
	}
}

// NewLocalCache creates a new filesystem-based cache.
func NewLocalCache(root string, opts ...LocalCacheOption) (*LocalCache, error) {
	c := &LocalCache{
		root: filepath.Clean(root),
		log:  logging.NewNopLogger(),
		fs:   afero.NewOsFs(),
	}

	for _, opt := range opts {
		opt(c)
	}

	// Ensure cache directory exists
	if err := c.fs.MkdirAll(c.root, 0o755); err != nil {
		return nil, errors.Wrap(err, "failed to create cache directory")
	}

	return c, nil
}

// Get retrieves a cached API dependency filesystem.
func (c *LocalCache) Get(dep v1alpha1.APIDependencies) (afero.Fs, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := c.calculateKey(dep)
	entryPath := c.entryPath(key)

	// Check if entry exists
	exists, err := afero.DirExists(c.fs, entryPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to check cache entry")
	}
	if !exists {
		return nil, os.ErrNotExist
	}

	// Create a new filesystem rooted at the cache entry
	cachedFS := afero.NewBasePathFs(c.fs, entryPath)

	c.log.Debug("Cache hit", "key", key)
	return cachedFS, nil
}

// Store saves an API dependency filesystem to the cache.
func (c *LocalCache) Store(dep v1alpha1.APIDependencies, srcFS afero.Fs) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := c.calculateKey(dep)
	entryPath := c.entryPath(key)

	// Clean any existing entry
	if err := c.fs.RemoveAll(entryPath); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to clean existing cache entry")
	}

	// Create entry directory
	if err := c.fs.MkdirAll(entryPath, 0o755); err != nil {
		return errors.Wrap(err, "failed to create cache entry directory")
	}

	// Copy files from source filesystem to cache
	// Create a base path filesystem for the destination
	destFS := afero.NewBasePathFs(c.fs, entryPath)
	if err := filesystem.CopyFilesBetweenFs(srcFS, destFS); err != nil {
		// Clean up on error
		_ = c.fs.RemoveAll(entryPath)
		return errors.Wrap(err, "failed to copy files to cache")
	}

	c.log.Debug("Stored in cache", "key", key)
	return nil
}

// calculateKey generates a cache key for the given dependency.
func (c *LocalCache) calculateKey(dep v1alpha1.APIDependencies) string {
	h := sha256.New()

	// Include cache version
	fmt.Fprintf(h, "version:%s\n", cacheVersion) //nolint:errcheck // hash.Hash never returns errors

	// Include dependency type
	fmt.Fprintf(h, "type:%s\n", dep.Type) //nolint:errcheck // hash.Hash never returns errors

	// Add source-specific information
	switch {
	case dep.Git != nil:
		fmt.Fprintf(h, "source:git\n")                  //nolint:errcheck // hash.Hash never returns errors
		fmt.Fprintf(h, "repo:%s\n", dep.Git.Repository) //nolint:errcheck // hash.Hash never returns errors
		fmt.Fprintf(h, "ref:%s\n", dep.Git.Ref)         //nolint:errcheck // hash.Hash never returns errors
		fmt.Fprintf(h, "path:%s\n", dep.Git.Path)       //nolint:errcheck // hash.Hash never returns errors
	case dep.HTTP != nil:
		fmt.Fprintf(h, "source:http\n")          //nolint:errcheck // hash.Hash never returns errors
		fmt.Fprintf(h, "url:%s\n", dep.HTTP.URL) //nolint:errcheck // hash.Hash never returns errors
	}

	return hex.EncodeToString(h.Sum(nil))[:16]
}

// entryPath returns the filesystem path for a cache entry.
func (c *LocalCache) entryPath(key string) string {
	return filepath.Join(c.root, key)
}
