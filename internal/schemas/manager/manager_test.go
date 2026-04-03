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
		lock          *lock
		existingFiles map[string]string // files to write into the FS before calling Add
		gen           generator.Interface
		src           Source

		expectedLock  *lock
		expectedFiles map[string]string
		expectErr     bool
	}{
		// Version matches, files tracked in lock, and files exist on disk: skip.
		"AlreadyGenerated": {
			lock: &lock{
				Packages: map[string]string{
					"xpkg.upbound.io/my-org/my-pkg": "v1.0.0",
				},
				Files: map[string][]string{
					"xpkg.upbound.io/my-org/my-pkg": {"mock/already-exists"},
				},
			},
			existingFiles: map[string]string{
				"mock/already-exists": "pre-existing content",
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
				Files: map[string][]string{
					"xpkg.upbound.io/my-org/my-pkg": {"mock/already-exists"},
				},
			},
			expectedFiles: map[string]string{
				"mock/already-exists": "pre-existing content",
			},
		},
		// Version matches, files tracked in lock, but files were deleted: regenerate.
		"DeletedModels": {
			lock: &lock{
				Packages: map[string]string{
					"xpkg.upbound.io/my-org/my-pkg": "v1.0.0",
				},
				Files: map[string][]string{
					"xpkg.upbound.io/my-org/my-pkg": {"mock/was-deleted"},
				},
			},
			// Note: "mock/was-deleted" is NOT written to existingFiles, simulating deletion.
			gen: &mockGenerator{
				files: map[string]string{
					"regenerated": "regenerated content",
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
				Files: map[string][]string{
					"xpkg.upbound.io/my-org/my-pkg": {"mock/regenerated"},
				},
			},
			expectedFiles: map[string]string{
				"mock/regenerated": "regenerated content",
			},
		},
		// Version matches but no files are tracked (old lock format): regenerate.
		"OldLockNoFileTracking": {
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
				version: "v1.0.0",
			},
			expectedLock: &lock{
				Packages: map[string]string{
					"xpkg.upbound.io/my-org/my-pkg": "v1.0.0",
				},
				Files: map[string][]string{
					"xpkg.upbound.io/my-org/my-pkg": {"mock/should-exist"},
				},
			},
			expectedFiles: map[string]string{
				"mock/should-exist": "does get created",
			},
		},
		// No lock at all: generate and start tracking files.
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
				Files: map[string][]string{
					"xpkg.upbound.io/my-org/my-pkg": {"mock/should-exist"},
				},
			},
			expectedFiles: map[string]string{
				"mock/should-exist": "does get created",
			},
		},
		// Version changed: regenerate and update file tracking.
		"VersionUpdated": {
			lock: &lock{
				Packages: map[string]string{
					"xpkg.upbound.io/my-org/my-pkg": "v1.0.0",
				},
				Files: map[string][]string{
					"xpkg.upbound.io/my-org/my-pkg": {"mock/old-file"},
				},
			},
			// No existingFiles: old files may or may not be on disk; version
			// mismatch alone is sufficient to trigger regeneration.
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
				Files: map[string][]string{
					"xpkg.upbound.io/my-org/my-pkg": {"mock/should-exist"},
				},
			},
			expectedFiles: map[string]string{
				"mock/should-exist": "does get created",
			},
		},
		"PackagedSource": {
			lock: &lock{
				Packages: make(map[string]string),
				Files:    make(map[string][]string),
			},
			src: &mockPackagedSource{
				mockSource: mockSource{
					id:      "xpkg.upbound.io/my-org/my-pkg",
					version: "v1.1.0",
				},
				files: map[string]string{
					"should-exist": "does get created",
				},
			},
			expectedLock: &lock{
				Packages: map[string]string{
					"xpkg.upbound.io/my-org/my-pkg": "v1.1.0",
				},
				Files: map[string][]string{
					"xpkg.upbound.io/my-org/my-pkg": {"mock/should-exist"},
				},
			},
			expectedFiles: map[string]string{
				"mock/should-exist": "does get created",
			},
			// Generator intentionally left nil since it should not be called.
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

			// Write any pre-existing files into the FS.
			for path, content := range tc.existingFiles {
				err := afero.WriteFile(testFS, path, []byte(content), 0o600)
				assert.NilError(t, err)
			}

			err := m.Add(t.Context(), tc.src)
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

func (g *mockGenerator) GenerateFromCRD(_ context.Context, _ afero.Fs, _ runner.SchemaRunner) (afero.Fs, error) {
	fs := afero.NewMemMapFs()
	for path, contents := range g.files {
		if err := afero.WriteFile(fs, path, []byte(contents), 0o600); err != nil {
			return nil, err
		}
	}
	return fs, nil
}

func (g *mockGenerator) GenerateFromOpenAPI(_ context.Context, _ afero.Fs, _ runner.SchemaRunner) (afero.Fs, error) {
	return nil, nil
}

type mockSource struct {
	id      string
	version string
}

func (s *mockSource) ID() string {
	return s.id
}

func (s *mockSource) Version(_ context.Context) (string, error) {
	return s.version, nil
}

func (s *mockSource) Resources(_ context.Context) (afero.Fs, error) {
	return nil, nil
}

func (s *mockSource) Type() SourceType {
	return SourceTypeCRD
}

type mockPackagedSource struct {
	mockSource
	files map[string]string
}

func (s *mockPackagedSource) Schemas() (map[string]afero.Fs, error) {
	fs := afero.NewMemMapFs()
	for path, contents := range s.files {
		if err := afero.WriteFile(fs, path, []byte(contents), 0o600); err != nil {
			return nil, err
		}
	}

	return map[string]afero.Fs{
		"mock": fs,
	}, nil
}

var _ PackagedSource = &mockPackagedSource{}
