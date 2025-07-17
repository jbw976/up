// Copyright 2025 Upbound Inc.
// All rights reserved

package ai

import (
	"context"
	"embed"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/pkg/apis/project/v1alpha1"
)

var (
	//go:embed all:testdata/fake-project-claude
	projectClaude embed.FS
	//go:embed all:testdata/fake-project-codex
	projectCodex embed.FS
	//go:embed all:testdata/fake-project-gemini
	projectGemini embed.FS
)

// TestRuleCmd_Run tests the Run method of the ruleCmd struct.
func TestRuleCmd_Run(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		gemini        bool
		claude        bool
		codex         bool
		fs            embed.FS
		path          string
		expectedFiles []string
		err           error
	}{
		"Gemini": {
			fs:            projectGemini,
			path:          "testdata/fake-project-gemini",
			gemini:        true,
			expectedFiles: []string{"GEMINI.md", "settings.json", "upbound.yaml"},
			err:           nil,
		},
		"Claude": {
			fs:            projectClaude,
			path:          "testdata/fake-project-claude",
			claude:        true,
			expectedFiles: []string{"CLAUDE.md", "settings.json", ".mcp.json", "upbound.yaml"},
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
			c := &rulesCmd{
				ProjectFile: "upbound.yaml",
				projFS:      projFS,
				proj:        proj,
			}

			printer := upterm.DefaultObjPrinter
			printer.Quiet = true
			err = c.Run(context.Background(), printer)

			if tc.err == nil {
				generatedFiles := []os.FileInfo{}
				// recurse the directories and pull files
				afero.Walk(projFS, ".", func(path string, info fs.FileInfo, err error) error {
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

type TestWriter struct {
	t *testing.T
}

func (w *TestWriter) Write(b []byte) (int, error) {
	out := strings.TrimRight(string(b), "\n")
	w.t.Log(out)
	return len(b), nil
}

func TestGeminiTemplate(t *testing.T) {
	r := &rulesCmd{
		proj: &v1alpha1.Project{
			ObjectMeta: v1.ObjectMeta{
				Name: "test",
			},
		},
	}
	r.generateGeminiTemplates()
}
