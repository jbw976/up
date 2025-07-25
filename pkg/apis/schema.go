// Copyright 2025 Upbound Inc.
// All rights reserved

package apis

import (
	"context"
	"embed"

	"golang.org/x/sync/errgroup"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/internal/xpkg/dep/manager"
	"github.com/upbound/up/internal/xpkg/schemagenerator"
	"github.com/upbound/up/internal/xpkg/schemarunner"
)

// Embed the CRDs folder into the binary.
//
//go:embed crds/*
var crdsFS embed.FS

// GenerateSchema will generate meta apis schemas.
func GenerateSchema(ctx context.Context, m *manager.Manager, sr schemarunner.SchemaRunner) error {
	schemaFS, err := filesystem.EmbedCopyOnWriteFs(crdsFS)
	if err != nil {
		return err
	}
	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		var err error
		kfs, err := schemagenerator.GenerateSchemaKcl(ctx, schemaFS, []string{}, sr)
		if err != nil {
			return err
		}

		if err := m.AddModels("kcl", kfs); err != nil {
			return err
		}
		return err
	})

	eg.Go(func() error {
		var err error
		pfs, err := schemagenerator.GenerateSchemaPython(ctx, schemaFS, []string{}, sr)
		if err != nil {
			return err
		}

		if err := m.AddModels("python", pfs); err != nil {
			return err
		}
		return err
	})

	eg.Go(func() error {
		var err error
		gofs, err := schemagenerator.GenerateSchemaGo(ctx, schemaFS, []string{}, sr)
		if err != nil {
			return err
		}

		if err := m.AddModels("go", gofs); err != nil {
			return err
		}
		return err
	})

	eg.Go(func() error {
		var err error
		jsonfs, err := schemagenerator.GenerateSchemaJSON(ctx, schemaFS, []string{}, sr)
		if err != nil {
			return err
		}

		if err := m.AddModels("json", jsonfs); err != nil {
			return err
		}
		return err
	})

	if err := eg.Wait(); err != nil {
		return errors.Wrap(err, "unable to generate meta schemas")
	}

	return nil
}
