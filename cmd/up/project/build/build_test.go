// Copyright 2025 Upbound Inc.
// All rights reserved

package build

import (
	"context"
	"embed"
	"fmt"
	"io"
	"maps"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/spf13/afero"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	xpmetav1 "github.com/crossplane/crossplane/v2/apis/pkg/meta/v1"
	xpkgv1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"

	"github.com/upbound/up/cmd/up/project/common"
	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/schemas/generator"
	"github.com/upbound/up/internal/schemas/runner"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/internal/xpkg"
	xpkgmarshaler "github.com/upbound/up/internal/xpkg/dep/marshaler/xpkg"
	"github.com/upbound/up/internal/xpkg/dep/resolver/image"
	"github.com/upbound/up/internal/xpkg/functions"
)

var (
	//go:embed testdata/configuration-getting-started/**
	configurationGettingStarted embed.FS

	//go:embed testdata/projectv1alpha1-embedded-functions/**
	projectv1alpha1EmbeddedFunctions embed.FS

	//go:embed testdata/projectv2alpha1-embedded-functions/**
	projectv2alpha1EmbeddedFunctions embed.FS

	//go:embed testdata/xrdv2/**
	xrdv2 embed.FS

	//go:embed testdata/packages/*
	packagesFS embed.FS
)

func TestBuild(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		projFS                  afero.Fs
		outputFile              string
		expectedFunctions       []*xpmetav1.Function
		expectedAnnotatedLayers map[string]bool
		expectedObjectCount     int
		expectedLabels          func(c *Cmd) map[string]string
	}{
		"XRDV2": {
			projFS: afero.NewBasePathFs(
				afero.FromIOFS{FS: xrdv2},
				"testdata/xrdv2",
			),
			outputFile:        "_output/xrd-v2.uppkg",
			expectedFunctions: nil,
			// 1 APIs = 1 XRDs + 1 compositions.
			expectedObjectCount: 2,
			expectedAnnotatedLayers: map[string]bool{
				xpkg.PackageAnnotation:  true,
				xpkg.ExamplesAnnotation: false,
				"schema.mock":           true,
			},
			expectedLabels: func(c *Cmd) map[string]string {
				return common.ImageLabels(c)
			},
		},
		"ConfigurationOnly": {
			projFS: afero.NewBasePathFs(
				afero.FromIOFS{FS: configurationGettingStarted},
				"testdata/configuration-getting-started",
			),
			outputFile:        "_output/configuration-getting-started.uppkg",
			expectedFunctions: nil,
			// 8 APIs = 8 XRDs + 8 compositions.
			expectedObjectCount: 16,
			expectedAnnotatedLayers: map[string]bool{
				xpkg.PackageAnnotation:  true,
				xpkg.ExamplesAnnotation: true,
				"schema.mock":           true,
			},
			expectedLabels: func(c *Cmd) map[string]string {
				return common.ImageLabels(c)
			},
		},
		"EmbeddedFunctionsWithProjectV1alpha1": {
			projFS: afero.NewBasePathFs(
				afero.FromIOFS{FS: projectv1alpha1EmbeddedFunctions},
				"testdata/projectv1alpha1-embedded-functions",
			),
			outputFile: "_output/project-embedded-functions.uppkg",
			expectedFunctions: []*xpmetav1.Function{
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: xpmetav1.SchemeGroupVersion.String(),
						Kind:       xpmetav1.FunctionKind,
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "project-embedded-functions-xcluster",
						Annotations: map[string]string{
							"meta.crossplane.io/maintainer":  "Upbound <support@upbound.io>",
							"meta.crossplane.io/source":      "github.com/upbound/project-getting-started",
							"meta.crossplane.io/license":     "Apache-2.0",
							"meta.crossplane.io/description": "Function xcluster from project project-embedded-functions",
						},
					},
					Spec: xpmetav1.FunctionSpec{
						MetaSpec: xpmetav1.MetaSpec{
							Capabilities: []string{
								xpmetav1.FunctionCapabilityComposition,
								xpmetav1.FunctionCapabilityOperation,
							},
						},
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: xpmetav1.SchemeGroupVersion.String(),
						Kind:       xpmetav1.FunctionKind,
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "project-embedded-functions-xnetwork",
						Annotations: map[string]string{
							"meta.crossplane.io/maintainer":  "Upbound <support@upbound.io>",
							"meta.crossplane.io/source":      "github.com/upbound/project-getting-started",
							"meta.crossplane.io/license":     "Apache-2.0",
							"meta.crossplane.io/description": "Function xnetwork from project project-embedded-functions",
						},
					},
					Spec: xpmetav1.FunctionSpec{
						MetaSpec: xpmetav1.MetaSpec{
							Capabilities: []string{
								xpmetav1.FunctionCapabilityComposition,
								xpmetav1.FunctionCapabilityOperation,
							},
						},
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: xpmetav1.SchemeGroupVersion.String(),
						Kind:       xpmetav1.FunctionKind,
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "project-embedded-functions-xsubnetwork",
						Annotations: map[string]string{
							"meta.crossplane.io/maintainer":  "Upbound <support@upbound.io>",
							"meta.crossplane.io/source":      "github.com/upbound/project-getting-started",
							"meta.crossplane.io/license":     "Apache-2.0",
							"meta.crossplane.io/description": "Function xsubnetwork from project project-embedded-functions",
						},
					},
					Spec: xpmetav1.FunctionSpec{
						MetaSpec: xpmetav1.MetaSpec{
							Capabilities: []string{
								xpmetav1.FunctionCapabilityComposition,
								xpmetav1.FunctionCapabilityOperation,
							},
						},
					},
				},
			},
			// 3 APIs = 3 XRDs + 3 compositions.
			expectedObjectCount: 6,
			expectedAnnotatedLayers: map[string]bool{
				xpkg.PackageAnnotation:  true,
				xpkg.ExamplesAnnotation: false, // no-examples expected
				"schema.mock":           true,
			},
			expectedLabels: func(c *Cmd) map[string]string {
				return common.ImageLabels(c)
			},
		},
		"EmbeddedFunctionsWithProjectV2alpha1": {
			projFS: afero.NewBasePathFs(
				afero.FromIOFS{FS: projectv2alpha1EmbeddedFunctions},
				"testdata/projectv2alpha1-embedded-functions",
			),
			outputFile: "_output/project-embedded-functions.uppkg",
			expectedFunctions: []*xpmetav1.Function{
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: xpmetav1.SchemeGroupVersion.String(),
						Kind:       xpmetav1.FunctionKind,
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "project-embedded-functions-xcluster",
						Annotations: map[string]string{
							"meta.crossplane.io/maintainer":  "Upbound <support@upbound.io>",
							"meta.crossplane.io/source":      "github.com/upbound/project-getting-started",
							"meta.crossplane.io/license":     "Apache-2.0",
							"meta.crossplane.io/description": "Function xcluster from project project-embedded-functions",
							"meta.upbound.io/team":           "platform-engineering",
							"meta.upbound.io/env":            "testing",
						},
					},
					Spec: xpmetav1.FunctionSpec{
						MetaSpec: xpmetav1.MetaSpec{
							Capabilities: []string{
								xpmetav1.FunctionCapabilityComposition,
								xpmetav1.FunctionCapabilityOperation,
							},
						},
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: xpmetav1.SchemeGroupVersion.String(),
						Kind:       xpmetav1.FunctionKind,
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "project-embedded-functions-xnetwork",
						Annotations: map[string]string{
							"meta.crossplane.io/maintainer":  "Upbound <support@upbound.io>",
							"meta.crossplane.io/source":      "github.com/upbound/project-getting-started",
							"meta.crossplane.io/license":     "Apache-2.0",
							"meta.crossplane.io/description": "Function xnetwork from project project-embedded-functions",
							"meta.upbound.io/team":           "platform-engineering",
							"meta.upbound.io/env":            "testing",
						},
					},
					Spec: xpmetav1.FunctionSpec{
						MetaSpec: xpmetav1.MetaSpec{
							Capabilities: []string{
								xpmetav1.FunctionCapabilityComposition,
								xpmetav1.FunctionCapabilityOperation,
							},
						},
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: xpmetav1.SchemeGroupVersion.String(),
						Kind:       xpmetav1.FunctionKind,
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "project-embedded-functions-xsubnetwork",
						Annotations: map[string]string{
							"meta.crossplane.io/maintainer":  "Upbound <support@upbound.io>",
							"meta.crossplane.io/source":      "github.com/upbound/project-getting-started",
							"meta.crossplane.io/license":     "Apache-2.0",
							"meta.crossplane.io/description": "Function xsubnetwork from project project-embedded-functions",
							"meta.upbound.io/team":           "platform-engineering",
							"meta.upbound.io/env":            "testing",
						},
					},
					Spec: xpmetav1.FunctionSpec{
						MetaSpec: xpmetav1.MetaSpec{
							Capabilities: []string{
								xpmetav1.FunctionCapabilityComposition,
								xpmetav1.FunctionCapabilityOperation,
							},
						},
					},
				},
			},
			// 3 XRDs + 3 compositions + 3 operations.
			expectedObjectCount: 9,
			expectedAnnotatedLayers: map[string]bool{
				xpkg.PackageAnnotation:  true,
				xpkg.ExamplesAnnotation: false, // no-examples expected
				"schema.mock":           true,
			},
			expectedLabels: func(c *Cmd) map[string]string {
				return common.ImageLabels(c)
			},
		},
	}

	for testName, tc := range tcs {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			outFS := afero.NewMemMapFs()

			// Create mock fetcher that holds the images
			testPkgFS := afero.NewBasePathFs(afero.FromIOFS{FS: packagesFS}, "testdata/packages")

			// Create an in-memory overlay so the builder can write to the
			// project FS (e.g., as part of schema generation).
			projFS := filesystem.MemOverlay(tc.projFS)
			prj, err := project.Parse(projFS, "upbound.yaml")
			assert.NilError(t, err)
			prj.Default()

			cchFS := afero.NewBasePathFs(outFS, "/cache")
			ep, err := url.Parse("https://donotuse.example.com")
			assert.NilError(t, err)
			upCtx := &upbound.Context{
				Domain:           &url.URL{},
				RegistryEndpoint: ep,
			}
			mgr, err := project.NewDependencyManager(upCtx, prj, projFS,
				project.WithCacheFS(cchFS),
				project.WithFetcher(&image.FSFetcher{FS: testPkgFS}),
				project.WithSchemaGenerators([]generator.Interface{mockGenerator{}}),
			)
			assert.NilError(t, err)

			c := &Cmd{
				ProjectFile:  "upbound.yaml",
				OutputDir:    "_output",
				NoBuildCache: true,

				projFS:             projFS,
				outputFS:           outFS,
				functionIdentifier: functions.FakeIdentifier,
				concurrency:        1,

				m:    mgr,
				proj: prj,
			}

			// Build the package.
			printer := upterm.NewTestPrinter()
			err = c.Run(t.Context(), upCtx, printer)
			assert.NilError(t, err)

			// List the built packages load them from the output file.
			cfgTag, err := name.NewTag(fmt.Sprintf("%s:%s", c.proj.Spec.Repository, project.ConfigurationTag))
			assert.NilError(t, err)
			opener := func() (io.ReadCloser, error) {
				return outFS.Open(tc.outputFile)
			}
			mfst, err := tarball.LoadManifest(opener)
			assert.NilError(t, err)

			var (
				fnImages = make(map[name.Repository][]v1.Image)
				cfgImage v1.Image
			)
			for _, desc := range mfst {
				if slices.Contains(desc.RepoTags, cfgTag.String()) {
					cfgImage, err = tarball.Image(opener, &cfgTag)
					assert.NilError(t, err)

					configFile, err := cfgImage.ConfigFile()
					assert.NilError(t, err)

					cfgImage, err = mutate.Config(cfgImage, configFile.Config)
					assert.NilError(t, err)

					cfgImage, err = xpkg.AnnotateImage(cfgImage)
					if err != nil {
						t.Fatalf("Failed to annotate image: %v", err)
					}

					// Check for annotations in the image manifest layers
					manifest, err := cfgImage.Manifest()
					assert.NilError(t, err)

					foundLayers := map[string]bool{
						xpkg.PackageAnnotation:  false,
						xpkg.ExamplesAnnotation: false,
						// Schema layer from our mock generator.
						"schema.mock": false,
					}

					// Iterate over manifest layers to find annotations
					for _, layer := range manifest.Layers {
						if value, ok := layer.Annotations[xpkg.AnnotationKey]; ok {
							// Mark the layer as found if it's an expected annotation
							if _, expected := foundLayers[value]; expected {
								foundLayers[value] = true
							}
						}
					}

					assert.DeepEqual(t, tc.expectedAnnotatedLayers, foundLayers)

					cfgFile, err := cfgImage.ConfigFile()
					assert.NilError(t, err)

					expectedLabels := tc.expectedLabels(c)

					for key, expectedValue := range expectedLabels {
						actualValue, exists := cfgFile.Config.Labels[key]
						assert.Assert(t, exists, "Label %s not found", key)
						assert.Equal(t, expectedValue, actualValue, "Label %s value mismatch", key)
					}
				} else {
					fnTag, err := name.NewTag(desc.RepoTags[0])
					assert.NilError(t, err)

					fnImage, err := tarball.Image(opener, &fnTag)
					assert.NilError(t, err)

					configFile, err := fnImage.ConfigFile()
					assert.NilError(t, err)

					fnImage, err = mutate.Config(fnImage, configFile.Config)
					assert.NilError(t, err)

					fnImage, err = xpkg.AnnotateImage(fnImage)
					if err != nil {
						t.Fatalf("Failed to annotate image: %v", err)
					}

					fnImages[fnTag.Repository] = append(fnImages[fnTag.Repository], fnImage)
				}
			}

			// Validate the function packages and collect the dependencies that
			// should have been generated for the configuration.
			var fnDeps []xpmetav1.Dependency
			assert.Equal(t, len(tc.expectedFunctions), len(fnImages))
			for repo, images := range fnImages {
				// There should be two images, one for each of the two default
				// architectures.
				assert.Assert(t, cmp.Len(images, 2))
				image := images[0]

				m, err := xpkgmarshaler.NewMarshaler()
				assert.NilError(t, err)
				pkg, err := m.FromImage(xpkg.Image{
					Image: image,
				})
				assert.NilError(t, err)

				linter := xpkg.NewFunctionLinter()
				err = linter.Lint(&PackageAdapter{pkg})
				assert.NilError(t, err)

				// Build an index so we know the digest of the desired
				// dependency.
				idx, _, err := xpkg.BuildIndex(images...)
				assert.NilError(t, err)
				dgst, err := idx.Digest()
				assert.NilError(t, err)
				fnDeps = append(fnDeps, xpmetav1.Dependency{
					APIVersion: ptr.To(xpkgv1.FunctionGroupVersionKind.GroupVersion().String()),
					Kind:       ptr.To(xpkgv1.FunctionKind),
					Package:    ptr.To(repo.String()),
					Version:    dgst.String(),
				})

				fnMeta, ok := pkg.Meta().(*xpmetav1.Function)
				assert.Assert(t, ok, "unexpected metadata type for function")
				assert.Assert(t, cmp.Contains(tc.expectedFunctions, fnMeta))
			}

			// Validate the configuration package.
			m, err := xpkgmarshaler.NewMarshaler()
			assert.NilError(t, err)
			pkg, err := m.FromImage(xpkg.Image{
				Image: cfgImage,
			})
			assert.NilError(t, err)

			linter := xpkg.NewConfigurationLinter()
			err = linter.Lint(&PackageAdapter{pkg})
			assert.NilError(t, err)

			cfgMeta, ok := pkg.Meta().(*xpmetav1.Configuration)
			assert.Assert(t, ok, "unexpected metadata type for configuration")
			assert.DeepEqual(t, cfgMeta.TypeMeta, metav1.TypeMeta{
				APIVersion: xpmetav1.SchemeGroupVersion.String(),
				Kind:       xpmetav1.ConfigurationKind,
			})
			expectedAnnotations := map[string]string{
				"meta.crossplane.io/maintainer":  c.proj.Spec.Maintainer,
				"meta.crossplane.io/source":      c.proj.Spec.Source,
				"meta.crossplane.io/license":     c.proj.Spec.License,
				"meta.crossplane.io/description": c.proj.Spec.Description,
				"meta.crossplane.io/readme":      c.proj.Spec.Readme,
			}
			maps.Copy(expectedAnnotations, c.proj.Spec.Annotations)
			assert.DeepEqual(t, cfgMeta.ObjectMeta, metav1.ObjectMeta{
				Name:        c.proj.Name,
				Annotations: expectedAnnotations,
			})
			// Our project doesn't have a Crossplane constraint, so we should
			// get the default.
			vproj, _ := project.ParseWithVersion(projFS, "upbound.yaml")
			if vproj.IsV2() {
				assert.DeepEqual(t, cfgMeta.Spec.Crossplane, &xpmetav1.CrossplaneConstraints{
					Version: ">=v2.0.0-rc.0",
				})
			} else {
				assert.DeepEqual(t, cfgMeta.Spec.Crossplane, &xpmetav1.CrossplaneConstraints{
					Version: ">=v1.19.0 || >=v2.0.0-rc.0",
				})
			}

			// Validate that the configuration depends on all the project
			// dependencies and the embedded functions.
			assert.Assert(t, cmp.Len(cfgMeta.Spec.DependsOn, len(c.proj.Spec.DependsOn)+len(fnImages)))
			for _, dep := range c.proj.Spec.DependsOn {
				assert.Assert(t, cmp.Contains(cfgMeta.Spec.DependsOn, dep))
			}
			for _, dep := range fnDeps {
				assert.Assert(t, cmp.Contains(cfgMeta.Spec.DependsOn, dep))
			}

			objs := pkg.Objects()
			assert.Assert(t, cmp.Len(objs, tc.expectedObjectCount))
		})
	}
}

type mockGenerator struct{}

func (g mockGenerator) Language() string {
	return "mock"
}

func (g mockGenerator) GenerateFromCRD(_ context.Context, fs afero.Fs, _ runner.SchemaRunner) (afero.Fs, error) {
	return fs, nil
}

func (g mockGenerator) GenerateFromOpenAPI(_ context.Context, _ afero.Fs, _ runner.SchemaRunner) (afero.Fs, error) {
	return nil, nil
}

type TestWriter struct {
	t *testing.T
}

func (w *TestWriter) Write(b []byte) (int, error) {
	out := strings.TrimRight(string(b), "\n")
	w.t.Log(out)
	return len(b), nil
}

// PackageAdapter translates a `ParsedPackage` from the xpkg marshaler into a
// `linter.Package` so we can lint it.
type PackageAdapter struct {
	wrap *xpkgmarshaler.ParsedPackage
}

func (pkg *PackageAdapter) GetMeta() []runtime.Object {
	return []runtime.Object{pkg.wrap.Meta()}
}

func (pkg *PackageAdapter) GetObjects() []runtime.Object {
	return pkg.wrap.Objects()
}
