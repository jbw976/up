// Copyright 2025 Upbound Inc.
// All rights reserved

package composition

import (
	"context"
	"embed"
	"io/fs"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/spf13/afero"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"

	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/xpkg/dep/resolver/image"
)

var (
	//go:embed testdata/projectA/**
	projectAFS embed.FS

	//go:embed testdata/projectB/**
	projectBFS embed.FS

	//go:embed testdata/projectC/**
	projectCFS embed.FS

	//go:embed testdata/packages/*
	packagesFS embed.FS

	//go:embed testdata/packagesB/*
	packagesBFS embed.FS
)

func TestNewComposition(t *testing.T) {
	type want struct {
		composition *v1.Composition
	}

	cases := map[string]struct {
		name       string
		plural     string
		packages   afero.Fs
		embeddedFS embed.FS
		want       want
	}{
		"CompositionWithLabelsAndName": {
			name:       "xyz",
			embeddedFS: projectAFS,
			packages:   afero.NewBasePathFs(afero.FromIOFS{FS: packagesFS}, "testdata/packages"),
			want: want{
				composition: &v1.Composition{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Composition",
						APIVersion: "apiextensions.crossplane.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "xyz.xnetworks.aws.platform.upbound.io",
						Labels: map[string]string{
							"cloud": "aws",
							"type":  "network",
						},
					},
					Spec: v1.CompositionSpec{
						CompositeTypeRef: v1.TypeReference{
							APIVersion: "aws.platform.upbound.io/v1alpha1", // Expected API version
							Kind:       "XNetwork",                         // Expected kind
						},
						Mode: ptr.To(v1.CompositionModePipeline),
						Pipeline: []v1.PipelineStep{
							{
								Step: "crossplane-contrib-function-kcl",
								FunctionRef: v1.FunctionReference{
									Name: "crossplane-contrib-function-kcl",
								},
								// Precomputed expected RawExtension for KCLInput
								Input: &runtime.RawExtension{
									Raw: []byte(kclTemplate),
								},
							},
							{
								Step: "crossplane-contrib-function-go-templating",
								FunctionRef: v1.FunctionReference{
									Name: "crossplane-contrib-function-go-templating",
								},
								// Precomputed expected RawExtension for Go-Template
								Input: &runtime.RawExtension{
									Raw: []byte(goTemplate),
								},
							},
							{
								Step: "crossplane-contrib-function-patch-and-transform",
								FunctionRef: v1.FunctionReference{
									Name: "crossplane-contrib-function-patch-and-transform",
								},
								// Precomputed expected RawExtension for Patch-and-Transform
								Input: &runtime.RawExtension{
									Raw: []byte(patTemplate),
								},
							},
							{
								Step: "crossplane-contrib-function-auto-ready",
								FunctionRef: v1.FunctionReference{
									Name: "crossplane-contrib-function-auto-ready",
								},
							},
						},
					},
				},
			},
		},
		"CompositionWithoutLabels": {
			embeddedFS: projectBFS,
			packages:   afero.NewBasePathFs(afero.FromIOFS{FS: packagesBFS}, "testdata/packagesB"),
			want: want{
				composition: &v1.Composition{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Composition",
						APIVersion: "apiextensions.crossplane.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "xnetworks.aws.platform.upbound.io",
					},
					Spec: v1.CompositionSpec{
						CompositeTypeRef: v1.TypeReference{
							APIVersion: "aws.platform.upbound.io/v1alpha1", // Expected API version
							Kind:       "XNetwork",                         // Expected kind
						},
						Mode: ptr.To(v1.CompositionModePipeline),
						Pipeline: []v1.PipelineStep{
							{
								Step: "crossplane-contrib-function-auto-ready",
								FunctionRef: v1.FunctionReference{
									Name: "crossplane-contrib-function-auto-ready",
								},
							},
						},
					},
				},
			},
		},
		"CompositionWithCustomPlural": {
			plural:     "Xpostgreses",
			embeddedFS: projectBFS,
			packages:   afero.NewBasePathFs(afero.FromIOFS{FS: packagesBFS}, "testdata/packagesB"),
			want: want{
				composition: &v1.Composition{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Composition",
						APIVersion: "apiextensions.crossplane.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "xpostgreses.aws.platform.upbound.io",
					},
					Spec: v1.CompositionSpec{
						CompositeTypeRef: v1.TypeReference{
							APIVersion: "aws.platform.upbound.io/v1alpha1", // Expected API version
							Kind:       "XNetwork",                         // Expected kind
						},
						Mode: ptr.To(v1.CompositionModePipeline),
						Pipeline: []v1.PipelineStep{
							{
								Step: "crossplane-contrib-function-auto-ready",
								FunctionRef: v1.FunctionReference{
									Name: "crossplane-contrib-function-auto-ready",
								},
							},
						},
					},
				},
			},
		},
		"CompositionFromXRD": {
			embeddedFS: projectCFS,
			packages:   afero.NewBasePathFs(afero.FromIOFS{FS: packagesBFS}, "testdata/packagesB"),
			want: want{
				composition: &v1.Composition{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Composition",
						APIVersion: "apiextensions.crossplane.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "xnetworks.aws.platform.upbound.io",
					},
					Spec: v1.CompositionSpec{
						CompositeTypeRef: v1.TypeReference{
							APIVersion: "aws.platform.upbound.io/v1alpha1", // Expected API version
							Kind:       "XNetwork",                         // Expected kind from plural
						},
						Mode: ptr.To(v1.CompositionModePipeline),
						Pipeline: []v1.PipelineStep{
							{
								Step: "crossplane-contrib-function-auto-ready",
								FunctionRef: v1.FunctionReference{
									Name: "crossplane-contrib-function-auto-ready",
								},
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			outFS := afero.NewMemMapFs()
			// Set up a mock cache directory in Afero
			cchFS := afero.NewBasePathFs(outFS, "/cache")

			// Embed test data into projectFS
			projFS := afero.NewMemMapFs()
			err := embedToAferoFS(tc.embeddedFS, projFS, "testdata", "/")
			assert.NilError(t, err)

			// Parse project config
			proj, err := project.Parse(projFS, "/upbound.yaml")
			assert.NilError(t, err)
			proj.Default()

			ep, err := url.Parse("https://donotuse.example.com")
			assert.NilError(t, err)
			upCtx := &upbound.Context{
				Domain:           &url.URL{},
				RegistryEndpoint: ep,
			}
			dm, err := project.NewDependencyManager(upCtx, proj, projFS,
				project.WithFetcher(&image.FSFetcher{FS: tc.packages}),
				project.WithSchemaGenerators(nil),
				project.WithCacheFS(cchFS),
				project.WithProjectFile("/upbound.yaml"),
			)
			assert.NilError(t, err)

			generateCmd := generateCmd{
				Name:        tc.name,
				Plural:      tc.plural,
				Resource:    "/test.yaml",
				ProjectFile: "/upbound.yaml",
				proj:        proj,
				projFS:      projFS,
				apisFS:      projFS,
				depManager:  dm,
			}

			// Call newComposition and check results
			got, _, err := generateCmd.newComposition(context.Background())
			assert.NilError(t, err)

			// Compare the result with the expected composition
			if diff := cmp.Diff(got, tc.want.composition, cmpopts.IgnoreUnexported(v1.Composition{})); diff != "" {
				t.Errorf("NewComposition() composition: -got, +want:\n%s", diff)
			}
		})
	}
}

// embedToAferoFS walks through an embedded FS and writes files to Afero FS.
func embedToAferoFS(embeddedFS embed.FS, aferoFS afero.Fs, sourceDir string, targetDir string) error {
	err := fs.WalkDir(embeddedFS, sourceDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Skip directories
		if d.IsDir() {
			return nil
		}
		// Read file content from the embedded FS
		data, err := embeddedFS.ReadFile(path)
		if err != nil {
			return err
		}
		// Write the file to the Afero filesystem at the target path
		targetPath := filepath.Join(targetDir, filepath.Base(path))
		err = afero.WriteFile(aferoFS, targetPath, data, 0o644)
		if err != nil {
			return err
		}
		return nil
	})
	return err
}
