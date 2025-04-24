// Copyright 2025 Upbound Inc.
// All rights reserved

package test

import (
	"context"
	"embed"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/internal/xpkg/dep/cache"
	"github.com/upbound/up/internal/xpkg/dep/manager"
	"github.com/upbound/up/internal/xpkg/dep/resolver/image"
	"github.com/upbound/up/internal/xpkg/schemarunner"
	"github.com/upbound/up/internal/xpkg/workspace"
)

var (
	//go:embed testdata/project-embedded-functions/**
	projectEmbeddedFunctions embed.FS

	//go:embed testdata/packages/*
	packagesFS embed.FS

	//go:embed testdata/project-embedded-functions/.up/**
	modelsFS embed.FS
)

func TestGenerateCmd_Run(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		language      string
		name          string
		expectedFiles []string
		err           error
	}{
		"LanguageKcl": {
			name:          "testkcl",
			language:      "kcl",
			expectedFiles: []string{"model", "main.k", "kcl.mod", "kcl.mod.lock"},
			err:           nil,
		},
		"LanguagePython": {
			name:          "testpython",
			language:      "python",
			expectedFiles: []string{"model", "main.py", "requirements.txt"},
			err:           nil,
		},
	}

	for testName, tc := range tcs {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()
			mockRunner := MockSchemaRunner{}

			outFS := afero.NewMemMapFs()
			tempProjDir, err := afero.TempDir(afero.NewOsFs(), os.TempDir(), "projFS")
			assert.NilError(t, err)
			defer os.RemoveAll(tempProjDir)

			projFS := afero.NewBasePathFs(afero.NewOsFs(), tempProjDir)
			srcFS := afero.NewBasePathFs(afero.FromIOFS{FS: projectEmbeddedFunctions}, "testdata/project-embedded-functions")

			err = filesystem.CopyFilesBetweenFs(srcFS, projFS)
			assert.NilError(t, err)

			ws, err := workspace.New("/", workspace.WithFS(outFS), workspace.WithPermissiveParser())
			assert.NilError(t, err)
			err = ws.Parse(context.Background())
			assert.NilError(t, err)

			cch, err := cache.NewLocal("/cache", cache.WithFS(outFS))
			assert.NilError(t, err)

			testPkgFS := afero.NewBasePathFs(afero.FromIOFS{FS: packagesFS}, "testdata/packages")
			testModelsFS := afero.NewBasePathFs(afero.FromIOFS{FS: modelsFS}, "testdata/project-embedded-functions/.up")
			r := image.NewResolver(
				image.WithFetcher(
					&image.FSFetcher{FS: testPkgFS},
				),
			)

			mgr, err := manager.New(
				manager.WithCache(cch),
				manager.WithResolver(r),
			)
			assert.NilError(t, err)

			ws, err = workspace.New("/",
				workspace.WithFS(projFS), // Use the copied projFS here
				workspace.WithPermissiveParser(),
			)
			assert.NilError(t, err)
			err = ws.Parse(context.Background())
			assert.NilError(t, err)

			proj, err := project.Parse(projFS, "upbound.yaml")
			assert.NilError(t, err)
			proj.Default()

			// Use BasePathFs for testFS, scoped to the temp directories
			testFS := afero.NewBasePathFs(projFS, filepath.Join("/tests", tc.name))

			// Setup the generateCmd with mock dependencies
			c := &generateCmd{
				ProjectFile:  "upbound.yaml",
				projFS:       projFS,
				testFS:       testFS,
				modelsFS:     testModelsFS,
				Language:     tc.language,
				Name:         tc.name,
				testName:     tc.name,
				m:            mgr,
				ws:           ws,
				schemaRunner: mockRunner,
			}

			err = c.Run(context.Background(), upterm.DefaultObjPrinter)
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

type MockSchemaRunner struct{}

func (m MockSchemaRunner) Generate(_ context.Context, fs afero.Fs, _ string, _ string, imageName string, _ []string, _ ...schemarunner.Option) error {
	// Simulate generation for KCL schema files
	if strings.Contains(imageName, "kcl") { // Check for KCL-specific marker, if any
		// Create the main KCL schema file
		kclOutputPath := "models/io/upbound/dev/meta/v1alpha1/compositiontest.k"
		_ = fs.MkdirAll("models/io/upbound/dev/meta/v1alpha1/", os.ModePerm)
		if err := afero.WriteFile(fs, kclOutputPath, []byte("mock KCL content"), os.ModePerm); err != nil {
			return err
		}

		// Create the additional k8s folder and a file inside
		k8sOutputPath := "models/k8s/sample_k8s_resource.k"
		_ = fs.MkdirAll("models/k8s/", os.ModePerm)
		return afero.WriteFile(fs, k8sOutputPath, []byte("mock K8s content"), os.ModePerm)
	}
	// Simulate generation for Python schema files
	outputPath := "models/workdir/platform_acme_co_v1alpha1_subnetwork/io/k8s/apimachinery/pkg/apis/meta/v1.py"
	_ = fs.MkdirAll("models/workdir/platform_acme_co_v1alpha1_subnetwork/io/k8s/apimachinery/pkg/apis/meta/", os.ModePerm)
	return afero.WriteFile(fs, outputPath, []byte("mock Python content"), os.ModePerm)
}
