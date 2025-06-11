// Copyright 2025 Upbound Inc.
// All rights reserved

package manager

import (
	"context"
	"io/fs"
	"testing"

	"github.com/spf13/afero"
	"gotest.tools/v3/assert"

	"github.com/upbound/up/internal/schemas/generator"
	"github.com/upbound/up/internal/schemas/runner"
)

func TestManager_Add(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		lock *lock
		gen  generator.Interface
		src  Source

		expectedLock  *lock
		expectedFiles map[string]string
		expectErr     bool
	}{
		"AlreadyGenerated": {
			lock: &lock{
				Packages: map[string]string{
					"xpkg.upbound.io/my-org/my-pkg": "v1.0.0",
				},
			},
			gen: &mockGenerator{
				files: map[string]string{
					"should-not-exist": "does not get created",
				},
			},
			src: &mockSource{
				id:      "xpkg.upbound.io/my-org/my-pkg",
				version: "v1.0.0",
			},
			expectedLock: &lock{
				Packages: map[string]string{
					"xpkg.upbound.io/my-org/my-pkg": "v1.0.0",
				},
			},
			expectedFiles: map[string]string{},
		},
		"EmptyLock": {
			gen: &mockGenerator{
				files: map[string]string{
					"should-exist": "does get created",
				},
			},
			src: &mockSource{
				id:      "xpkg.upbound.io/my-org/my-pkg",
				version: "v1.0.0",
			},
			expectedLock: &lock{
				Packages: map[string]string{
					"xpkg.upbound.io/my-org/my-pkg": "v1.0.0",
				},
			},
			expectedFiles: map[string]string{
				"mock/should-exist": "does get created",
			},
		},
		"VersionUpdated": {
			lock: &lock{
				Packages: map[string]string{
					"xpkg.upbound.io/my-org/my-pkg": "v1.0.0",
				},
			},
			gen: &mockGenerator{
				files: map[string]string{
					"should-exist": "does get created",
				},
			},
			src: &mockSource{
				id:      "xpkg.upbound.io/my-org/my-pkg",
				version: "v1.1.0",
			},
			expectedLock: &lock{
				Packages: map[string]string{
					"xpkg.upbound.io/my-org/my-pkg": "v1.1.0",
				},
			},
			expectedFiles: map[string]string{
				"mock/should-exist": "does get created",
			},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			testFS := afero.NewMemMapFs()

			m := New(testFS, []generator.Interface{tc.gen}, nil)
			if tc.lock != nil {
				err := m.updateLock(tc.lock)
				assert.NilError(t, err)
			}

			err := m.Add(context.Background(), tc.src, nil)
			if tc.expectErr {
				assert.Assert(t, err != nil)
				return
			}

			// Ensure the expected files have the correct contents, and that no
			// extra files were written.
			_ = afero.Walk(testFS, ".", func(path string, info fs.FileInfo, err error) error {
				assert.NilError(t, err)
				if info.Name() == lockFileName {
					return nil
				}
				if info.IsDir() {
					return nil
				}

				want, ok := tc.expectedFiles[path]
				if !ok {
					t.Errorf("unexpected file %q generated", path)
				}

				got, err := afero.ReadFile(testFS, path)
				assert.NilError(t, err)
				assert.Equal(t, string(got), want)

				return nil
			})

			gotLock, err := m.getLock()
			assert.NilError(t, err)
			assert.DeepEqual(t, tc.expectedLock, gotLock)
		})
	}
}

type mockGenerator struct {
	files map[string]string
}

func (g *mockGenerator) Language() string {
	return "mock"
}

func (g *mockGenerator) Generate(_ context.Context, _ afero.Fs, _ []string, _ runner.SchemaRunner) (afero.Fs, error) {
	fs := afero.NewMemMapFs()
	for path, contents := range g.files {
		if err := afero.WriteFile(fs, path, []byte(contents), 0o600); err != nil {
			return nil, err
		}
	}
	return fs, nil
}

type mockSource struct {
	id      string
	version string
}

func (s *mockSource) ID() string {
	return s.id
}

func (s *mockSource) Version() (string, error) {
	return s.version, nil
}

func (s *mockSource) Resources() (afero.Fs, error) {
	return nil, nil
}
