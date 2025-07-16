// Copyright 2025 Upbound Inc.
// All rights reserved

package ai

import (
	"context"
	"embed"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/upterm"
)

var (
	//go:embed testdata/fake-project/**
	projectEmbeddedFunctions embed.FS
)

// TestRuleCmd_Run tests the Run method of the ruleCmd struct.
func TestRuleCmd_Run(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		gemini        bool
		expectedFiles []string
		err           error
	}{
		"Gemini": {
			gemini:        true,
			expectedFiles: []string{"GEMINI.md", ".gemini", "upbound.yaml"},
			err:           nil,
		},
	}

	for testName, tc := range tcs {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			tempProjDir := t.TempDir()
			projFS := afero.NewBasePathFs(afero.NewOsFs(), tempProjDir)
			srcFS := afero.NewBasePathFs(afero.FromIOFS{FS: projectEmbeddedFunctions}, "testdata/fake-project")
			err := filesystem.CopyFilesBetweenFs(srcFS, projFS)
			assert.NilError(t, err)

			proj, err := project.Parse(projFS, "upbound.yaml")
			assert.NilError(t, err)
			proj.Default()

			assert.NilError(t, err)

			// Setup the rulesCmd with mock dependencies
			c := &rulesCmd{
				ProjectFile: "upbound.yaml",
				projFS:      projFS,
				proj:        proj,
			}

			printer := upterm.DefaultObjPrinter
			printer.Quiet = true
			err = c.Run(context.Background(), printer)

			if tc.err == nil {
				generatedFiles, err := afero.ReadDir(projFS, ".")
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
