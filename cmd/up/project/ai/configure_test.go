// Copyright 2025 Upbound Inc.
// All rights reserved

package ai

import (
	"embed"
	"io/fs"
	"os"
	"testing"

	"github.com/spf13/afero"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/upterm"
)

var (
	//go:embed all:testdata/fake-project-claude
	projectClaude embed.FS
	//go:embed all:testdata/fake-project-cursor
	projectCursor embed.FS
	//go:embed all:testdata/fake-project-gemini
	projectGemini embed.FS
)

// TestRuleCmd_Run tests the Run method of the ruleCmd struct.
// NOTE(tnthornton) this test currently validates the existence of the files.
// TODO(tnthornton) add tests for file contents.
func TestRuleCmd_Run(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		fs            embed.FS
		path          string
		expectedFiles []string
		err           error
	}{
		"Gemini": {
			fs:            projectGemini,
			path:          "testdata/fake-project-gemini",
			expectedFiles: []string{"GEMINI.md", "settings.json", "upbound.yaml"},
			err:           nil,
		},
		"Claude": {
			fs:            projectClaude,
			path:          "testdata/fake-project-claude",
			expectedFiles: []string{"CLAUDE.md", "settings.json", ".mcp.json", "upbound.yaml"},
			err:           nil,
		},
		"Cursor": {
			fs:            projectCursor,
			path:          "testdata/fake-project-cursor",
			expectedFiles: []string{"project.mdc", "mcp.json", "upbound.yaml"},
			err:           nil,
		},
	}

	for testName, tc := range tcs {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			tempProjDir := t.TempDir()
			projFS := afero.NewBasePathFs(afero.NewOsFs(), tempProjDir)
			srcFS := afero.NewBasePathFs(afero.FromIOFS{FS: tc.fs}, tc.path)
			err := filesystem.CopyFilesBetweenFs(srcFS, projFS)
			assert.NilError(t, err)

			proj, err := project.Parse(projFS, "upbound.yaml")
			assert.NilError(t, err)
			proj.Default()

			assert.NilError(t, err)

			// Setup the rulesCmd with mock dependencies
			c := &configureToolsCmd{
				ProjectFile: "upbound.yaml",
				projFS:      projFS,
				proj:        proj,
			}

			printer := upterm.DefaultObjPrinter
			printer.Quiet = true
			err = c.Run(printer)

			if tc.err == nil {
				generatedFiles := []os.FileInfo{}
				// recurse the directories and pull files
				afero.Walk(projFS, ".", func(_ string, info fs.FileInfo, err error) error {
					if !info.IsDir() {
						generatedFiles = append(generatedFiles, info)
					}
					return err
				})

				assert.NilError(t, err)
				assert.Assert(t, cmp.Len(generatedFiles, len(tc.expectedFiles)))

				for _, info := range generatedFiles {
					assert.Assert(t, cmp.Contains(tc.expectedFiles, info.Name()))
				}
			}
		})
	}
}
