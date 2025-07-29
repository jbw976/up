// Copyright 2025 Upbound Inc.
// All rights reserved

package function

import (
	"embed"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	apiextv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	opsv1alpha1 "github.com/crossplane/crossplane/apis/ops/v1alpha1"

	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/internal/xpkg/dep/resolver/image"
	"github.com/upbound/up/internal/yaml"
)

var (
	//go:embed testdata/project-embedded-functions/**
	projectEmbeddedFunctions embed.FS

	//go:embed testdata/packages/*
	packagesFS embed.FS
)

// TestGenerateCmd_Run tests the Run method of the generateCmd struct.
func TestGenerateCmd_Run(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		language     string
		name         string
		pipelinePath string

		expectedPipeline runtime.Object
		expectedFiles    []string
		err              error
	}{
		"LanguageKcl": {
			name:          "fn1",
			language:      "kcl",
			expectedFiles: []string{"model", "main.k", "kcl.mod", "kcl.mod.lock"},
			err:           nil,
		},
		"WithCompositionPath": {
			name:         "fn2",
			language:     "kcl",
			pipelinePath: "apis/primitives/XNetwork/composition.yaml",
			expectedPipeline: &apiextv1.Composition{
				TypeMeta: metav1.TypeMeta{
					APIVersion: apiextv1.CompositionGroupVersionKind.GroupVersion().String(),
					Kind:       apiextv1.CompositionKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "xnetworks.platform.acme.co",
				},
				Spec: apiextv1.CompositionSpec{
					CompositeTypeRef: apiextv1.TypeReference{
						APIVersion: "platform.acme.co/v1alpha1",
						Kind:       "XNetwork",
					},
					Mode: apiextv1.CompositionModePipeline,
					Pipeline: []apiextv1.PipelineStep{
						{
							Step: "fn2",
							FunctionRef: apiextv1.FunctionReference{
								Name: "awg-getting-startedfn2",
							},
						},
						{
							Step: "automatically-detect-ready-composed-resources",
							FunctionRef: apiextv1.FunctionReference{
								Name: "crossplane-contrib-function-auto-ready",
							},
						},
					},
				},
			},
			expectedFiles: []string{"model", "main.k", "kcl.mod", "kcl.mod.lock"},
			err:           nil,
		},
		"WithOperationPath": {
			name:         "fn2",
			language:     "kcl",
			pipelinePath: "operations/my-operation/operation.yaml",
			expectedPipeline: &opsv1alpha1.CronOperation{
				TypeMeta: metav1.TypeMeta{
					APIVersion: opsv1alpha1.CronOperationGroupVersionKind.GroupVersion().String(),
					Kind:       opsv1alpha1.CronOperationKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-operation",
				},
				Spec: opsv1alpha1.CronOperationSpec{
					Schedule: "0 * * * *",
					OperationTemplate: opsv1alpha1.OperationTemplate{
						Spec: opsv1alpha1.OperationSpec{
							Mode: opsv1alpha1.OperationModePipeline,
							// function-dummy step from the original pipeline
							// should be removed.
							Pipeline: []opsv1alpha1.PipelineStep{{
								Step: "fn2",
								FunctionRef: opsv1alpha1.FunctionReference{
									Name: "awg-getting-startedfn2",
								},
							}},
							RetryLimit: ptr.To(int64(3)),
						},
					},
				},
			},
			expectedFiles: []string{"model", "main.k", "kcl.mod", "kcl.mod.lock"},
			err:           nil,
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
		"LanguageGoTemplating": {
			name:          "fn5",
			language:      "go-templating",
			expectedFiles: []string{"00-prelude.yaml.gotmpl", "01-compose.yaml.gotmpl"},
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

			// Use BasePathFs for functionFS, scoped to the temp directories
			functionFS := afero.NewBasePathFs(projFS, filepath.Join("/functions", tc.name))

			// Setup the generateCmd with mock dependencies
			c := &generateCmd{
				ProjectFile:       "upbound.yaml",
				projFS:            projFS,
				proj:              proj,
				modelsFS:          testModelsFS,
				fsPath:            filepath.Join("/functions", tc.name),
				functionFS:        functionFS,
				Language:          tc.language,
				PipelinePath:      tc.pipelinePath,
				Name:              tc.name,
				projectRepository: "xpkg.upbound.io/awg/getting-started",
				m:                 mgr,
			}

			printer := upterm.DefaultObjPrinter
			printer.Quiet = true
			err = c.Run(t.Context(), printer)

			if tc.err == nil {
				assert.NilError(t, err)
			} else if err != nil {
				assert.Assert(t, strings.Contains(err.Error(), "DNS-1035"), "expected error message to mention DNS-1035 constraints")
			}

			if tc.pipelinePath != "" {
				gotYAML, err := afero.ReadFile(projFS, tc.pipelinePath)
				assert.NilError(t, err)

				wantYAML, err := yaml.Marshal(tc.expectedPipeline,
					yaml.RemoveField("spec.operationTemplate.metadata"),
					yaml.RemoveField("metadata.creationTimestamp"),
					yaml.RemoveField("status"),
				)
				assert.NilError(t, err)
				assert.DeepEqual(t, wantYAML, gotYAML)
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
