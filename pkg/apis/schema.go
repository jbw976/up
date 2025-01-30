// Copyright 2025 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package apis

import (
	"context"
	"embed"

	"github.com/spf13/afero"
	"golang.org/x/sync/errgroup"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/async"
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
	basePathFS := afero.NewCopyOnWriteFs(afero.NewBasePathFs(
		afero.FromIOFS{FS: crdsFS},
		"crds",
	), afero.NewMemMapFs())
	schemaFS := afero.NewCopyOnWriteFs(basePathFS, afero.NewMemMapFs())

	if err := async.WrapWithSuccessSpinners(func(_ async.EventChannel) error {
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

		return eg.Wait()
	}); err != nil {
		return errors.Wrap(err, "unable to generate meta schemas")
	}

	return nil
}
