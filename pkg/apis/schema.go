// Copyright 2025 Upbound Inc.
// All rights reserved

package apis

import (
	"context"
	"crypto/sha256"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"

	"github.com/spf13/afero"

	"github.com/upbound/up/internal/schemas/manager"
)

// Embed the CRDs folder into the binary.
//
//go:embed crds/*
var crdsFS embed.FS

// GenerateSchema will generate meta apis schemas.
func GenerateSchema(ctx context.Context, m *manager.Manager) error {
	return m.Add(ctx, &metaAPIsSource{fs: crdsFS})
}

type metaAPIsSource struct {
	fs embed.FS
}

func (f *metaAPIsSource) ID() string {
	return "up://apis"
}

func (f *metaAPIsSource) Version(_ context.Context) (string, error) {
	// Calculate a hash of all the yaml files in the filesystem. This isn't
	// super efficient, but is almost certainly faster than generating schemas
	// for the files.
	h := sha256.New()

	if err := fs.WalkDir(f.fs, "crds", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
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

func (f *metaAPIsSource) Resources(_ context.Context) (afero.Fs, error) {
	return afero.NewBasePathFs(afero.FromIOFS{FS: f.fs}, "crds"), nil
}

func (f *metaAPIsSource) Type() manager.SourceType {
	return manager.SourceTypeCRD
}
