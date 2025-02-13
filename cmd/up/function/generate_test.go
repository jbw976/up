// Copyright 2025 Upbound Inc.
// All rights reserved

package function

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/spf13/afero"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"sigs.k8s.io/yaml"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"

	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/xpkg"
	"github.com/upbound/up/internal/xpkg/dep/cache"
	"github.com/upbound/up/internal/xpkg/dep/manager"
	"github.com/upbound/up/internal/xpkg/dep/resolver/image"
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

// TestGenerateCmd_Run tests the Run method of the generateCmd struct.
func TestGenerateCmd_Run(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		language        string
		name            string
		compositionPath string
		expectedFiles   []string
		err             error
	}{
		"LanguageKcl": {
			name:          "fn1",
			language:      "kcl",
			expectedFiles: []string{"model", "main.k", "kcl.mod", "kcl.mod.lock"},
			err:           nil,
		},
		"WithCompositionPath": {
			name:            "fn2",
			language:        "kcl",
			compositionPath: "apis/primitives/XNetwork/composition.yaml",
			expectedFiles:   []string{"model", "main.k", "kcl.mod", "kcl.mod.lock"},
			err:             nil,
		},
		"LanguagePython": {
			name:          "fn3",
			language:      "python",
			expectedFiles: []string{"model", "main.py", "requirements.txt"},
			err:           nil,
		},
		"LanguageGo": {
			name:          "fn4",
			language:      "go",
			expectedFiles: []string{"fn.go", "fn_test.go", "go.mod", "go.sum", "main.go"},
			err:           nil,
		},
		"InvalidName": {
			name:          "apis/network/aws-yaml",
			language:      "python",
			expectedFiles: []string{},
			err:           fmt.Errorf("must meet DNS-1035 label constraints"), // General DNS-1035 error message
		},
	}

	for testName, tc := range tcs {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

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

			// Use BasePathFs for functionFS, scoped to the temp directories
			functionFS := afero.NewBasePathFs(projFS, filepath.Join("/functions", tc.name))

			// Setup the generateCmd with mock dependencies
			c := &generateCmd{
				ProjectFile:       "upbound.yaml",
				projFS:            projFS,
				proj:              proj,
				modelsFS:          testModelsFS,
				functionFS:        functionFS,
				Language:          tc.language,
				CompositionPath:   tc.compositionPath,
				Name:              tc.name,
				projectRepository: "xpkg.upbound.io/awg/getting-started",
				m:                 mgr,
				ws:                ws,
			}

			err = c.Run(context.Background())

			if tc.err == nil {
				assert.NilError(t, err)
			} else if err != nil {
				assert.Assert(t, strings.Contains(err.Error(), "DNS-1035"), "expected error message to mention DNS-1035 constraints")
			}

			if tc.compositionPath != "" {
				compYAML, err := afero.ReadFile(projFS, tc.compositionPath)
				assert.NilError(t, err)

				var comp v1.Composition
				err = yaml.Unmarshal(compYAML, &comp)
				assert.NilError(t, err)

				if len(comp.Spec.Pipeline) > 0 {
					step := comp.Spec.Pipeline[0]
					fnRepo := fmt.Sprintf("%s_%s", c.projectRepository, strings.ToLower(c.Name))
					ref, _ := name.ParseReference(fnRepo)
					assert.Equal(t, step.Step, c.Name, "expected pipeline step at index 0")
					assert.Equal(t, step.FunctionRef.Name, xpkg.ToDNSLabel(ref.Context().RepositoryStr()), "unexpected function reference in pipeline step index 0")
				} else {
					t.Error("expected at least one pipeline step, but found none")
				}
			}

			if tc.err == nil {
				generatedFiles, err := afero.ReadDir(functionFS, ".")
				assert.NilError(t, err)
				assert.Assert(t, cmp.Len(generatedFiles, len(tc.expectedFiles)))

				for _, info := range generatedFiles {
					assert.Assert(t, cmp.Contains(tc.expectedFiles, info.Name()))
				}
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
