// Copyright 2025 Upbound Inc.
// All rights reserved

package test

import (
	"embed"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/internal/xpkg/dep/resolver/image"
)

var (
	//go:embed testdata/project-embedded-functions/**
	projectEmbeddedFunctions embed.FS

	//go:embed testdata/packages/*
	packagesFS embed.FS
)

func TestGenerateCmd_Run(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		language      string
		name          string
		e2e           bool
		operation     bool
		expectedFiles []string
		err           error
	}{
		"CompositionTestKcl": {
			name:          "testkcl",
			language:      "kcl",
			e2e:           false,
			operation:     false,
			expectedFiles: []string{"model", "main.k", "kcl.mod", "kcl.mod.lock"},
			err:           nil,
		},
		"CompositionTestPython": {
			name:          "testpython",
			language:      "python",
			e2e:           false,
			operation:     false,
			expectedFiles: []string{"model", "main.py", "requirements.txt"},
			err:           nil,
		},
		"E2ETestKcl": {
			name:          "testkcl",
			language:      "kcl",
			e2e:           true,
			operation:     false,
			expectedFiles: []string{"model", "main.k", "kcl.mod", "kcl.mod.lock"},
			err:           nil,
		},
		"E2ETestPython": {
			name:          "testpython",
			language:      "python",
			e2e:           true,
			operation:     false,
			expectedFiles: []string{"model", "main.py", "requirements.txt"},
			err:           nil,
		},
		"OperationTestKcl": {
			name:          "testkcl",
			language:      "kcl",
			e2e:           false,
			operation:     true,
			expectedFiles: []string{"model", "main.k", "kcl.mod", "kcl.mod.lock"},
			err:           nil,
		},
		"OperationTestPython": {
			name:          "testpython",
			language:      "python",
			e2e:           false,
			operation:     true,
			expectedFiles: []string{"model", "main.py", "requirements.txt"},
			err:           nil,
		},
	}

	for testName, tc := range tcs {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			// Our symlinking implementation requires that the underlying
			// filesystem for the projFS is a real OS filesystem, so we can't
			// use an in-memory filesystem like we do in other tests.
			tempProjDir := t.TempDir()
			projFS := afero.NewBasePathFs(afero.NewOsFs(), tempProjDir)
			srcFS := afero.NewBasePathFs(afero.FromIOFS{FS: projectEmbeddedFunctions}, "testdata/project-embedded-functions")
			err := filesystem.CopyFilesBetweenFs(srcFS, projFS)
			assert.NilError(t, err)
			testModelsFS := afero.NewBasePathFs(projFS, ".up")

			outFS := afero.NewMemMapFs()
			testPkgFS := afero.NewBasePathFs(afero.FromIOFS{FS: packagesFS}, "testdata/packages")

			proj, err := project.Parse(projFS, "upbound.yaml")
			assert.NilError(t, err)
			proj.Default()

			cchFS := afero.NewBasePathFs(outFS, "/cache")
			ep, err := url.Parse("https://donotuse.example.com")
			assert.NilError(t, err)
			upCtx := &upbound.Context{
				Domain:           &url.URL{},
				RegistryEndpoint: ep,
			}
			mgr, err := project.NewDependencyManager(upCtx, proj, projFS,
				project.WithCacheFS(cchFS),
				project.WithFetcher(&image.FSFetcher{FS: testPkgFS}),
				project.WithSchemaGenerators(nil),
			)
			assert.NilError(t, err)

			// Determine test name and template base folder based on test type
			templateBaseFolder := "compositiontest"
			if tc.e2e {
				templateBaseFolder = "e2e"
			}
			if tc.operation {
				templateBaseFolder = "operationtest"
			}

			// Use BasePathFs for testFS, scoped to the temp directories
			testFS := afero.NewBasePathFs(afero.NewOsFs(), filepath.Join(tempProjDir, "tests", "test"))

			// Setup the generateCmd with mock dependencies
			c := &generateCmd{
				ProjectFile:        "upbound.yaml",
				projFS:             projFS,
				testFS:             testFS,
				modelsFS:           testModelsFS,
				Language:           tc.language,
				Name:               tc.name,
				E2E:                tc.e2e,
				Operation:          tc.operation,
				testName:           tc.name,
				templateBaseFolder: templateBaseFolder,
				m:                  mgr,
				proj:               proj,
			}

			err = c.Run(t.Context(), upterm.DefaultObjPrinter)
			if tc.err != nil {
				assert.ErrorContains(t, err, tc.err.Error())
				return
			}
			assert.NilError(t, err)

			// Verify generated files
			generatedFiles, err := afero.ReadDir(testFS, ".")
			assert.NilError(t, err)
			assert.Assert(t, cmp.Len(generatedFiles, len(tc.expectedFiles)))

			for _, info := range generatedFiles {
				assert.Assert(t, cmp.Contains(tc.expectedFiles, info.Name()))
			}
		})
	}
}

type TestWriter struct {
	t *testing.T
}

func (w *TestWriter) Write(b []byte) (int, error) {
	out := strings.TrimRight(string(b), "\n")
	w.t.Log(out)
	return len(b), nil
}
