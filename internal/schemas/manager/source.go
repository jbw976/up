// Copyright 2025 Upbound Inc.
// All rights reserved

package manager

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/helper/iofs"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/spf13/afero"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/internal/git"
	"github.com/upbound/up/internal/xpkg/dep/marshaler/xpkg"
	"github.com/upbound/up/internal/yaml"
	"github.com/upbound/up/pkg/apis/project/v2alpha1"
)

// SourceType represents the type of source.
type SourceType string

const (
	// SourceTypeCRD indicates a source containing CRDs/XRDs.
	SourceTypeCRD SourceType = "crd"
	// SourceTypeOpenAPI indicates a source containing OpenAPI specifications.
	SourceTypeOpenAPI SourceType = "openapi"
)

// Source is a source of resources for which schemas can be generated.
type Source interface {
	// ID returns a unique identifier for this source that does not change
	// across versions. For example, this could be an OCI repository.
	ID() string
	// Version returns a revision identifier for this source. For example, this
	// could be an OCI image tag or digest.
	Version(ctx context.Context) (string, error)
	// Resources returns a filesystem containing resources for which schemas
	// need to be generated. Each resource must be in its own file with the
	// extension .yml or .yaml. Resources may be Crossplane XRDs or Kubernetes
	// CRDs; all other kinds will be ignored.
	Resources(ctx context.Context) (afero.Fs, error)
	// Type returns the type of source, which determines which generators to use.
	Type() SourceType
}

// PackagedSource is a source of resources that includes pre-generated schemas.
type PackagedSource interface {
	// Schemas returns a map in which the keys are languages and the values are
	// filesystems containing schemas for the language.
	Schemas() (map[string]afero.Fs, error)
}

// calculateFilesystemHash calculates a SHA256 hash of the filesystem contents
// based on the source type (different file extensions for different types).
func calculateFilesystemHash(filesystem afero.Fs, sourceType SourceType) (string, error) {
	h := sha256.New()

	var extensions []string
	switch sourceType {
	case SourceTypeCRD:
		extensions = []string{".yaml", ".yml"}
	case SourceTypeOpenAPI:
		extensions = []string{".json"}
	default:
		// For unknown source types, include common schema file types
		extensions = []string{".yaml", ".yml", ".json"}
	}

	if err := afero.Walk(filesystem, ".", func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Check file extension
		ext := strings.ToLower(filepath.Ext(path))
		if !slices.Contains(extensions, ext) {
			return nil
		}

		f, err := filesystem.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()

		if _, err := io.Copy(h, f); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return "", err
	}

	sum := h.Sum(nil)
	return fmt.Sprintf("%x", sum), nil
}

// filterCRDs creates a new filesystem with CRDs filtered based on shouldFilterCRD.
// It only processes YAML files and filters out CRDs that match certain criteria.
func filterCRDs(sourceFS afero.Fs) (afero.Fs, error) {
	filteredFS := afero.NewMemMapFs()

	// Walk through all files and filter CRDs
	if err := afero.Walk(sourceFS, ".", func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Only process YAML files
		ext := filepath.Ext(path)
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		// Read the file
		content, err := afero.ReadFile(sourceFS, path)
		if err != nil {
			return err
		}

		// Check if this is a CRD that should be filtered out
		if shouldFilterCRD(content) {
			// Skip this CRD
			return nil
		}

		// Copy the file to the filtered filesystem
		if err := afero.WriteFile(filteredFS, path, content, info.Mode()); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return filteredFS, nil
}

// fsSource is a resource source backed by a filesystem.
type fsSource struct {
	fs afero.Fs
}

// ID returns the ID for an fsSource.
func (f *fsSource) ID() string {
	return "fs://" + filesystem.FullPath(f.fs, "/")
}

// Version returns the version for an fsSource.
func (f *fsSource) Version(_ context.Context) (string, error) {
	// Calculate a hash of all the yaml files in the filesystem. This isn't
	// super efficient, but is almost certainly faster than generating schemas
	// for all the files.
	return calculateFilesystemHash(f.fs, SourceTypeCRD)
}

// Resources returns the underlying filesystem, since it contains the resource
// files.
func (f *fsSource) Resources(_ context.Context) (afero.Fs, error) {
	return f.fs, nil
}

// Type returns the source type for fsSource.
func (f *fsSource) Type() SourceType {
	return SourceTypeCRD
}

// NewFSSource returns a new filesystem-backed resource source.
func NewFSSource(fs afero.Fs) Source {
	return &fsSource{fs: fs}
}

// xpkgSource is a source backed by a parsed xpkg.
type xpkgSource struct {
	pkg *xpkg.ParsedPackage
}

func (s *xpkgSource) ID() string {
	return "xpkg://" + s.pkg.Name()
}

func (s *xpkgSource) Version(_ context.Context) (string, error) {
	return s.pkg.SHA, nil
}

func (s *xpkgSource) Resources(_ context.Context) (afero.Fs, error) {
	fs := afero.NewMemMapFs()

	for i, obj := range s.pkg.Objects() {
		bs, err := yaml.Marshal(obj)
		if err != nil {
			return nil, err
		}
		if err := afero.WriteFile(fs, fmt.Sprintf("%d.yaml", i), bs, 0o600); err != nil {
			return nil, err
		}
	}

	return fs, nil
}

// Type returns the source type for xpkgSource.
func (s *xpkgSource) Type() SourceType {
	return SourceTypeCRD
}

// Schemas returns packaged schemas from the xpkg.
func (s *xpkgSource) Schemas() (map[string]afero.Fs, error) {
	return s.pkg.Schemas(), nil
}

// NewXpkgSource returns a new xpkg-backed resource source.
func NewXpkgSource(pkg *xpkg.ParsedPackage) Source {
	return &xpkgSource{pkg: pkg}
}

// shouldFilterCRD checks if a CRD should be filtered out based on its categories and schema.
// Returns true if the CRD has "crossplane" category combined with "managed", "store", or "provider".
// https://github.com/upbound/arch/pull/304
func shouldFilterCRD(content []byte) bool {
	var crd apiextensionsv1.CustomResourceDefinition
	if err := yaml.Unmarshal(content, &crd); err != nil {
		return false
	}

	if crd.Kind != "CustomResourceDefinition" {
		return false
	}

	hasCrossplane := false
	hasManaged := false
	// for storeConfig
	hasStore := false
	// for providerConfig, providerConfigUsage
	hasProvider := false

	for _, cat := range crd.Spec.Names.Categories {
		switch cat {
		case "crossplane":
			hasCrossplane = true
		case "managed":
			hasManaged = true
		case "store":
			hasStore = true
		case "provider":
			hasProvider = true
		}
	}

	// Filter out if crossplane is combined with managed, store, or provider
	return hasCrossplane && (hasManaged || hasStore || hasProvider)
}

const maxCloneAttempts = 3

// gitSource is a resource source that fetches directly from git repositories.
type gitSource struct {
	git          *v2alpha1.APIGitReference
	cloner       git.Cloner
	authProvider git.AuthProvider
	sourceType   SourceType
	fs           afero.Fs // cached filesystem
	commitSHA    string   // cached commit SHA
}

// ID returns a unique identifier for this git source.
func (g *gitSource) ID() string {
	id := fmt.Sprintf("git://%s", g.git.Repository)
	if g.git.Path != "" {
		id = fmt.Sprintf("%s/%s", id, g.git.Path)
	}
	return id
}

// Version returns the git commit SHA as the version identifier.
func (g *gitSource) Version(ctx context.Context) (string, error) {
	// If we already have the commit SHA cached, return it
	if g.commitSHA != "" {
		return g.commitSHA, nil
	}

	// Otherwise, fetch first to get the commit SHA
	if _, err := g.Resources(ctx); err != nil {
		return "", err
	}
	return g.commitSHA, nil
}

// Resources fetches and returns the filesystem containing resources.
func (g *gitSource) Resources(_ context.Context) (afero.Fs, error) {
	// Return cached filesystem if available
	if g.fs != nil {
		return g.fs, nil
	}

	ref := g.normalizeRef(g.git.Ref)

	var memFS billy.Filesystem
	var lastErr error

	// Retry logic for git operations
	for attempt := 1; attempt <= maxCloneAttempts; attempt++ {
		memFS = memfs.New()

		headRef, err := g.cloner.CloneRepository(
			memory.NewStorage(),
			memFS,
			g.authProvider,
			git.CloneOptions{
				Repo:    g.git.Repository,
				RefName: ref,
				Path:    g.git.Path,
			},
		)
		if err != nil {
			lastErr = errors.Wrapf(err, "clone attempt %d failed", attempt)
			continue
		}

		// Store the commit SHA
		if headRef != nil {
			g.commitSHA = headRef.Hash().String()
		}

		// Verify the clone was successful
		if err := g.verifyClone(memFS, g.git.Path); err != nil {
			lastErr = errors.Wrapf(err, "verification failed after attempt %d", attempt)
			continue
		}

		// All checks passed
		lastErr = nil
		break
	}

	if lastErr != nil {
		return nil, errors.Wrapf(lastErr, "failed to clone repository %s after %d attempts", g.git.Repository, maxCloneAttempts)
	}

	// Convert billy filesystem to afero using io/fs adapters
	resultFS := afero.NewMemMapFs()

	if err := filesystem.CopyFilesBetweenFs(afero.FromIOFS{FS: iofs.New(memFS)}, resultFS); err != nil {
		return nil, errors.Wrap(err, "failed to copy files from git repository")
	}

	// Apply filtering for CRD sources
	if g.sourceType == SourceTypeCRD {
		filteredFS, err := filterCRDs(resultFS)
		if err != nil {
			return nil, errors.Wrap(err, "failed to filter CRDs")
		}
		g.fs = filteredFS
	} else {
		g.fs = resultFS
	}

	return g.fs, nil
}

// Type returns the source type.
func (g *gitSource) Type() SourceType {
	return g.sourceType
}

// normalizeRef converts a git reference to a full ref format.
func (g *gitSource) normalizeRef(ref string) string {
	if ref == "" {
		return "refs/heads/main"
	}

	if git.CheckSHA256Hash(ref) {
		return ref
	}

	// Check if it's already a full ref
	if len(ref) > 5 && ref[:5] == "refs/" {
		return ref
	}

	// Check if it looks like a semantic version tag (e.g., v1.0.0, 1.2.3)
	if isVersionTag(ref) {
		return "refs/tags/" + ref
	}

	// Assume it's a branch name
	return "refs/heads/" + ref
}

// verifyClone checks that the cloned repository contains files at the specified path.
func (g *gitSource) verifyClone(fs billy.Filesystem, path string) error {
	files, err := fs.ReadDir(path)
	if err != nil {
		return errors.Wrapf(err, "cannot read cloned path %s", path)
	}

	if len(files) == 0 {
		return errors.Errorf("no files found in cloned path %s", path)
	}

	return nil
}

// isVersionTag checks if a reference looks like a version tag.
func isVersionTag(ref string) bool {
	// Check for common version patterns
	if len(ref) == 0 {
		return false
	}

	// Check if it starts with 'v' followed by a digit
	if ref[0] == 'v' && len(ref) > 1 && ref[1] >= '0' && ref[1] <= '9' {
		return true
	}

	// Check if it starts with a digit (e.g., "1.0.0")
	if ref[0] >= '0' && ref[0] <= '9' {
		return true
	}

	return false
}

// NewGitSource returns a new git-backed resource source.
func NewGitSource(dep v2alpha1.APIDependencies, cloner git.Cloner, authProvider git.AuthProvider) Source {
	sourceType := SourceTypeCRD
	if dep.Type == v2alpha1.APIDependencyTypeK8s {
		sourceType = SourceTypeOpenAPI
	}

	return &gitSource{
		git:          dep.Git,
		cloner:       cloner,
		authProvider: authProvider,
		sourceType:   sourceType,
	}
}

const (
	// defaultHTTPTimeout is the default timeout for HTTP requests.
	defaultHTTPTimeout = 1 * time.Minute
	// maxHTTPSize is the maximum size we'll download (100MB).
	maxHTTPSize = 100 * 1024 * 1024
)

// httpSource is a resource source that fetches directly from HTTP/HTTPS URLs.
// httpSource is a resource source that fetches directly from HTTP/HTTPS URLs.
type httpSource struct {
	http       *v2alpha1.APIHTTPReference
	client     *http.Client
	sourceType SourceType
	fs         afero.Fs // cached filesystem
}

// ID returns a unique identifier for this HTTP source.
func (h *httpSource) ID() string {
	return fmt.Sprintf("http://%s", h.http.URL)
}

// Version returns a hash of the content as the version identifier.
func (h *httpSource) Version(ctx context.Context) (string, error) {
	// If we already have the filesystem cached, calculate hash
	if h.fs != nil {
		return h.calculateHash()
	}

	// Otherwise, fetch first
	if _, err := h.Resources(ctx); err != nil {
		return "", err
	}
	return h.calculateHash()
}

// Resources fetches and returns the filesystem containing resources.
func (h *httpSource) Resources(ctx context.Context) (afero.Fs, error) {
	// Return cached filesystem if available
	if h.fs != nil {
		return h.fs, nil
	}

	// Validate URL
	u, err := url.Parse(h.http.URL)
	if err != nil {
		return nil, errors.Wrap(err, "invalid URL")
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, errors.Errorf("unsupported URL scheme: %s", u.Scheme)
	}

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.http.URL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}

	// Execute request
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch URL")
	}
	defer resp.Body.Close() //nolint:errcheck // nothing todo here

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Check content length
	if resp.ContentLength > maxHTTPSize {
		return nil, errors.Errorf("content too large: %d bytes (max: %d)", resp.ContentLength, maxHTTPSize)
	}

	// Read the content with size limit
	limitedReader := io.LimitReader(resp.Body, maxHTTPSize)
	content, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}

	// Create filesystem with the content
	resultFS := afero.NewMemMapFs()

	// Determine filename from URL or use default
	filename := h.getFilename(u)

	// Write content to filesystem
	if err := afero.WriteFile(resultFS, filename, content, 0o644); err != nil {
		return nil, errors.Wrap(err, "failed to write content to filesystem")
	}

	// Apply filtering for CRD sources
	if h.sourceType == SourceTypeCRD {
		filteredFS, err := filterCRDs(resultFS)
		if err != nil {
			return nil, errors.Wrap(err, "failed to filter CRDs")
		}
		h.fs = filteredFS
	} else {
		h.fs = resultFS
	}

	return h.fs, nil
}

// Type returns the source type.
func (h *httpSource) Type() SourceType {
	return h.sourceType
}

// calculateHash calculates a hash of the filesystem contents.
func (h *httpSource) calculateHash() (string, error) {
	return calculateFilesystemHash(h.fs, h.sourceType)
}

// getFilename extracts a filename from the URL or provides a default.
func (h *httpSource) getFilename(u *url.URL) string {
	// Try to get filename from path
	filename := path.Base(u.Path)

	// If no valid filename, use a default based on content
	if filename == "" || filename == "." || filename == "/" {
		// Check if it looks like a YAML file URL
		switch {
		case strings.Contains(u.String(), "yaml") || strings.Contains(u.String(), "yml"):
			filename = "crd.yaml"
		case h.sourceType == SourceTypeOpenAPI:
			filename = "openapi.json"
		default:
			filename = "crd"
		}
	}

	return filename
}

// NewHTTPSource returns a new HTTP-backed resource source.
func NewHTTPSource(dep v2alpha1.APIDependencies) Source {
	sourceType := SourceTypeCRD
	if dep.Type == v2alpha1.APIDependencyTypeK8s {
		sourceType = SourceTypeOpenAPI
	}

	return &httpSource{
		http: dep.HTTP,
		client: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
		sourceType: sourceType,
	}
}
