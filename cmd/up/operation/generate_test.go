// Copyright 2025 Upbound Inc.
// All rights reserved

package operation

import (
	"bytes"
	"embed"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/pterm/pterm"
	"github.com/spf13/afero"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"

	opsv1alpha1 "github.com/crossplane/crossplane/apis/ops/v1alpha1"
	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"

	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/xpkg/dep/resolver/image"
	"github.com/upbound/up/internal/yaml"
)

var (
	//go:embed testdata/packages/**
	packagesFS embed.FS

	//go:embed testdata/empty-project/**
	emptyProject embed.FS
)

func expectedInput() *runtime.RawExtension {
	y := `apiVersion: dummy.fn.crossplane.io/v1beta1
kind: Response
metadata: {}
response: {}`
	j, _ := yaml.YAMLToJSON([]byte(y))
	return &runtime.RawExtension{Raw: j}
}

func expectedOperation() *opsv1alpha1.Operation {
	return &opsv1alpha1.Operation{
		TypeMeta: metav1.TypeMeta{
			APIVersion: opsv1alpha1.OperationGroupVersionKind.GroupVersion().String(),
			Kind:       opsv1alpha1.OperationKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-operation",
		},
		Spec: opsv1alpha1.OperationSpec{
			RetryLimit: ptr.To(int64(3)),
			Mode:       opsv1alpha1.OperationModePipeline,
			Pipeline: []opsv1alpha1.PipelineStep{{
				Step: "crossplane-contrib-function-dummy",
				FunctionRef: opsv1alpha1.FunctionReference{
					Name: "crossplane-contrib-function-dummy",
				},
				Input: expectedInput(),
			}},
		},
	}
}

func expectedCronOperation() *opsv1alpha1.CronOperation {
	op := expectedOperation()
	return &opsv1alpha1.CronOperation{
		TypeMeta: metav1.TypeMeta{
			APIVersion: opsv1alpha1.CronOperationGroupVersionKind.GroupVersion().String(),
			Kind:       opsv1alpha1.CronOperationKind,
		},
		ObjectMeta: op.ObjectMeta,
		Spec: opsv1alpha1.CronOperationSpec{
			Schedule: "0 * * * *",
			OperationTemplate: opsv1alpha1.OperationTemplate{
				Spec: op.Spec,
			},
		},
	}
}

func expectedWatchOperation() *opsv1alpha1.WatchOperation {
	op := expectedOperation()
	return &opsv1alpha1.WatchOperation{
		TypeMeta: metav1.TypeMeta{
			APIVersion: opsv1alpha1.WatchOperationGroupVersionKind.GroupVersion().String(),
			Kind:       opsv1alpha1.WatchOperationKind,
		},
		ObjectMeta: op.ObjectMeta,
		Spec: opsv1alpha1.WatchOperationSpec{
			Watch: opsv1alpha1.WatchSpec{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
			},
			OperationTemplate: opsv1alpha1.OperationTemplate{
				Spec: op.Spec,
			},
		},
	}
}

func TestGenerateCmdRun(t *testing.T) {
	t.Parallel()

	expectedOperationYAML, err := yaml.Marshal(expectedOperation(),
		yaml.RemoveField("metadata.creationTimestamp"),
		yaml.RemoveField("spec.operationTemplate.metadata"),
		yaml.RemoveField("status"),
	)
	assert.NilError(t, err)
	expectedOperationJSON, err := yaml.YAMLToJSON(expectedOperationYAML)
	assert.NilError(t, err)

	expectedCronOperationYAML, err := yaml.Marshal(expectedCronOperation(),
		yaml.RemoveField("metadata.creationTimestamp"),
		yaml.RemoveField("spec.operationTemplate.metadata"),
		yaml.RemoveField("status"),
	)
	assert.NilError(t, err)
	expectedCronOperationJSON, err := yaml.YAMLToJSON(expectedCronOperationYAML)
	assert.NilError(t, err)

	expectedWatchOperationYAML, err := yaml.Marshal(expectedWatchOperation(),
		yaml.RemoveField("metadata.creationTimestamp"),
		yaml.RemoveField("spec.operationTemplate.metadata"),
		yaml.RemoveField("status"),
	)
	assert.NilError(t, err)
	expectedWatchOperationJSON, err := yaml.YAMLToJSON(expectedWatchOperationYAML)
	assert.NilError(t, err)

	type input struct {
		name   string
		output string
		path   string
		cron   string
		watch  watchSpec
	}

	type wantFile struct {
		name     string
		contents []byte
	}

	type want struct {
		stdout []byte
		file   *wantFile
		err    string
	}

	cases := map[string]struct {
		input input
		want  want
	}{
		"OneshotOutputToFile": {
			input: input{
				name:   "test-operation",
				output: "file",
				path:   "test-operation.yaml",
			},
			want: want{
				// The path looks weird here because we're not using a real
				// filesystem in the test.
				stdout: []byte("successfully created Operation and saved to /operations/test-operation.yaml/test-operation.yaml\n"),
				file: &wantFile{
					name:     "test-operation.yaml",
					contents: expectedOperationYAML,
				},
				err: "",
			},
		},
		"OneshotOutputYAML": {
			input: input{
				name:   "test-operation",
				output: "yaml",
			},
			want: want{
				// pterm adds two newlines in Println.
				stdout: append(expectedOperationYAML, '\n', '\n'),
				err:    "",
			},
		},
		"OneshotOutputJSON": {
			input: input{
				name:   "test-operation",
				output: "json",
			},
			want: want{
				// pterm adds two newlines in Println.
				stdout: append(expectedOperationJSON, '\n', '\n'),
				err:    "",
			},
		},
		"CronOutputToFile": {
			input: input{
				name:   "test-operation",
				output: "file",
				path:   "test-operation.yaml",
				cron:   "0 * * * *",
			},
			want: want{
				stdout: []byte("successfully created CronOperation and saved to /operations/test-operation.yaml/test-operation.yaml\n"),
				file: &wantFile{
					name:     "test-operation.yaml",
					contents: expectedCronOperationYAML,
				},
				err: "",
			},
		},
		"CronOutputYAML": {
			input: input{
				name:   "test-operation",
				output: "yaml",
				cron:   "0 * * * *",
			},
			want: want{
				// pterm adds two newlines in Println.
				stdout: append(expectedCronOperationYAML, '\n', '\n'),
				err:    "",
			},
		},
		"CronOutputJSON": {
			input: input{
				name:   "test-operation",
				output: "json",
				cron:   "0 * * * *",
			},
			want: want{
				// pterm adds two newlines in Println.
				stdout: append(expectedCronOperationJSON, '\n', '\n'),
				err:    "",
			},
		},
		"WatchOutputToFile": {
			input: input{
				name:   "test-operation",
				output: "file",
				path:   "test-operation.yaml",
				watch: watchSpec{
					GroupVersionKind: "apps/v1/Deployment",
					gvk: schema.GroupVersionKind{
						Group:   "apps",
						Version: "v1",
						Kind:    "Deployment",
					},
				},
			},
			want: want{
				stdout: []byte("successfully created WatchOperation and saved to /operations/test-operation.yaml/test-operation.yaml\n"),
				file: &wantFile{
					name:     "test-operation.yaml",
					contents: expectedWatchOperationYAML,
				},
				err: "",
			},
		},
		"WatchOutputYAML": {
			input: input{
				name:   "test-operation",
				output: "yaml",
				watch: watchSpec{
					GroupVersionKind: "apps/v1/Deployment",
					gvk: schema.GroupVersionKind{
						Group:   "apps",
						Version: "v1",
						Kind:    "Deployment",
					},
				},
			},
			want: want{
				// pterm adds two newlines in Println.
				stdout: append(expectedWatchOperationYAML, '\n', '\n'),
				err:    "",
			},
		},
		"WatchOutputJSON": {
			input: input{
				name:   "test-operation",
				output: "json",
				watch: watchSpec{
					GroupVersionKind: "apps/v1/Deployment",
					gvk: schema.GroupVersionKind{
						Group:   "apps",
						Version: "v1",
						Kind:    "Deployment",
					},
				},
			},
			want: want{
				// pterm adds two newlines in Println.
				stdout: append(expectedWatchOperationJSON, '\n', '\n'),
				err:    "",
			},
		},
		"InvalidOutput": {
			input: input{
				name:   "test-operation",
				output: "invalid",
			},
			want: want{
				err: "invalid output format specified",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			projFS := filesystem.MemOverlay(afero.NewBasePathFs(afero.FromIOFS{FS: emptyProject}, "testdata/empty-project"))
			proj, err := project.Parse(projFS, "/upbound.yaml")
			assert.NilError(t, err)
			proj.Default()

			ep, err := url.Parse("https://donotuse.example.com")
			assert.NilError(t, err)
			upCtx := &upbound.Context{
				Domain:           &url.URL{},
				RegistryEndpoint: ep,
			}
			pkgFS := afero.NewBasePathFs(afero.FromIOFS{FS: packagesFS}, "testdata/packages")
			dm, err := project.NewDependencyManager(upCtx, proj, projFS,
				project.WithFetcher(&image.FSFetcher{FS: pkgFS}),
				project.WithSchemaGenerators(nil),
				project.WithCacheFS(afero.NewMemMapFs()),
				project.WithProjectFile("/upbound.yaml"),
			)
			assert.NilError(t, err)

			cmd := &generateCmd{
				Name:       tc.input.name,
				Output:     tc.input.output,
				Path:       tc.input.path,
				Cron:       tc.input.cron,
				Watch:      tc.input.watch,
				Functions:  []string{"xpkg.upbound.io/crossplane-contrib/function-dummy"},
				proj:       proj,
				projFS:     projFS,
				opsFS:      afero.NewBasePathFs(projFS, filepath.Join("operations", tc.input.path)),
				depManager: dm,
			}

			var stdout bytes.Buffer
			err = cmd.Run(t.Context(), &pterm.BasicTextPrinter{Writer: &stdout})
			if tc.want.err != "" {
				assert.Error(t, err, tc.want.err)
				return
			}
			assert.DeepEqual(t, tc.want.stdout, stdout.Bytes())

			if tc.want.file != nil {
				gotContents, err := afero.ReadFile(cmd.opsFS, tc.want.file.name)
				assert.NilError(t, err)
				assert.DeepEqual(t, tc.want.file.contents, gotContents)
			}

			// Make sure we added the function-dummy dep to the project.
			assert.Assert(t, cmp.Contains(proj.Spec.DependsOn, pkgmetav1.Dependency{
				APIVersion: ptr.To("pkg.crossplane.io/v1"),
				Kind:       ptr.To("Function"),
				Package:    ptr.To("xpkg.upbound.io/crossplane-contrib/function-dummy"),
				Version:    ">=v0.0.0",
			}))
		})
	}
}
