// Copyright 2025 Upbound Inc.
// All rights reserved

package manager

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"

	"github.com/spf13/afero"

	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/internal/xpkg/dep/marshaler/xpkg"
	"github.com/upbound/up/internal/yaml"
)

// Source is a source of resources for which dependencies can be generated.
type Source interface {
	// ID returns a unique identifier for this source that does not change
	// across versions. For example, this could be an OCI repository.
	ID() string
	// Version returns a revision identifier for this source. For example, this
	// could be an OCI image tag or digest.
	Version() (string, error)
	// Resources returns a filesystem containing resources for which schemas
	// need to be generated. Each resource must be in its own file with the
	// extension .yml or .yaml. Resources may be Crossplane XRDs or Kubernetes
	// CRDs; all other kinds will be ignored.
	Resources() (afero.Fs, error)
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
func (f *fsSource) Version() (string, error) {
	// Calculate a hash of all the yaml files in the filesystem. This isn't
	// super efficient, but is almost certainly faster than generating schemas
	// for all the files.
	h := sha256.New()

	if err := afero.Walk(f.fs, ".", func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		// Ignore files without yaml extensions.
		ext := filepath.Ext(path)
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		f, err := f.fs.Open(path)
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

// Resources returns the underlying filesystem, since it contains the resource
// files.
func (f *fsSource) Resources() (afero.Fs, error) {
	return f.fs, nil
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

func (s *xpkgSource) Version() (string, error) {
	return s.pkg.SHA, nil
}

func (s *xpkgSource) Resources() (afero.Fs, error) {
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

// NewXpkgSource returns a new xpkg-backed resource source.
func NewXpkgSource(pkg *xpkg.ParsedPackage) Source {
	return &xpkgSource{pkg: pkg}
}
