// Copyright 2025 Upbound Inc.
// All rights reserved

package xpkg

import (
	"archive/tar"
	"context"
	"io"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/spf13/afero"
	"github.com/spf13/afero/tarfs"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/parser"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/upbound/up/internal/xpkg/parser/examples"
	"github.com/upbound/up/internal/xpkg/parser/yaml"
)

var (
	testCRD               []byte
	testAuth              []byte
	testPC                []byte
	testMetav1alpha1WAuth []byte
	testMetav1alpha1      []byte
	testMetav1WAuth       []byte
	testMetav1            []byte
	testController        []byte
	testHelmChart         []byte
	testEx1               []byte
	testEx2               []byte
	testEx3               []byte
	testEx4               []byte

	_ parser.Backend = &MockBackend{}

	defaultFilters = []parser.FilterFn{
		parser.SkipDirs(),
		parser.SkipNotYAML(),
		parser.SkipEmpty(),
	}
)

func init() {
	testCRD, _ = afero.ReadFile(afero.NewOsFs(), "testdata/providerconfigs.helm.crossplane.io.yaml")
	testPC, _ = afero.ReadFile(afero.NewOsFs(), "testdata/upbound.io_providerconfigs.yaml")
	testMetav1alpha1WAuth, _ = afero.ReadFile(afero.NewOsFs(), "testdata/provider_meta_v1alpha1_w_auth.yaml")
	testMetav1alpha1, _ = afero.ReadFile(afero.NewOsFs(), "testdata/provider_meta_v1alpha1.yaml")
	testMetav1WAuth, _ = afero.ReadFile(afero.NewOsFs(), "testdata/provider_meta_v1_w_auth.yaml")
	testMetav1, _ = afero.ReadFile(afero.NewOsFs(), "testdata/provider_meta_v1.yaml")
	testAuth, _ = afero.ReadFile(afero.NewOsFs(), "testdata/auth.yaml")
	testController, _ = afero.ReadFile(afero.NewOsFs(), "testdata/controller_meta.yaml")
	testHelmChart, _ = afero.ReadFile(afero.NewOsFs(), "testdata/chart.tgz")
	testEx1, _ = afero.ReadFile(afero.NewOsFs(), "testdata/examples/ec2/instance.yaml")
	testEx2, _ = afero.ReadFile(afero.NewOsFs(), "testdata/examples/ec2/internetgateway.yaml")
	testEx3, _ = afero.ReadFile(afero.NewOsFs(), "testdata/examples/ecr/repository.yaml")
	testEx4, _ = afero.ReadFile(afero.NewOsFs(), "testdata/examples/provider.yaml")
}

type MockBackend struct {
	MockInit func() (io.ReadCloser, error)
}

func NewMockInitFn(r io.ReadCloser, err error) func() (io.ReadCloser, error) {
	return func() (io.ReadCloser, error) { return r, err }
}

func (m *MockBackend) Init(_ context.Context, _ ...parser.BackendOption) (io.ReadCloser, error) {
	return m.MockInit()
}

var _ parser.Parser = &MockParser{}

type MockParser struct {
	MockParse func() (*parser.Package, error)
}

func NewMockParseFn(pkg *parser.Package, err error) func() (*parser.Package, error) {
	return func() (*parser.Package, error) { return pkg, err }
}

func (m *MockParser) Parse(context.Context, io.ReadCloser) (*parser.Package, error) {
	return m.MockParse()
}

func TestBuild(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		be parser.Backend
		ex parser.Backend
		p  parser.Parser
		e  *examples.Parser
	}
	cases := map[string]struct {
		reason string
		args   args
		want   error
	}{
		"ErrInitBackend": {
			reason: "Should return an error if we fail to initialize backend.",
			args: args{
				be: &MockBackend{
					MockInit: NewMockInitFn(nil, errBoom),
				},
			},
			want: errors.Wrap(errBoom, errInitBackend),
		},
		"ErrParse": {
			reason: "Should return an error if we fail to parse package.",
			args: args{
				be: parser.NewEchoBackend(""),
				ex: parser.NewEchoBackend(""),
				p: &MockParser{
					MockParse: NewMockParseFn(nil, errBoom),
				},
			},
			want: errors.Wrap(errBoom, errParserPackage),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			builder := New(tc.args.be, nil, tc.args.ex, nil, tc.args.p, tc.args.e, nil)

			_, _, err := builder.Build(context.TODO())

			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nBuild(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestBuildExamples(t *testing.T) {
	pkgp, _ := yaml.New()

	type withFsFn func() afero.Fs

	type args struct {
		rootDir     string
		examplesDir string
		fs          withFsFn
	}
	type want struct {
		pkgExists bool
		exExists  bool
		labels    []string
		err       error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SuccessNoExamples": {
			args: args{
				rootDir:     "/ws",
				examplesDir: "/ws/examples",
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					_ = fs.Mkdir("/ws", os.ModePerm)
					_ = fs.Mkdir("/ws/crds", os.ModePerm)
					_ = afero.WriteFile(fs, "/ws/crossplane.yaml", testMetav1alpha1, os.ModePerm)
					_ = afero.WriteFile(fs, "/ws/crds/crd.yaml", testCRD, os.ModePerm)
					return fs
				},
			},
			want: want{
				pkgExists: true,
				labels: []string{
					PackageAnnotation,
				},
			},
		},
		"SuccessExamplesAtRoot": {
			args: args{
				rootDir:     "/ws",
				examplesDir: "/ws/examples",
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					_ = fs.Mkdir("/ws", os.ModePerm)
					_ = afero.WriteFile(fs, "/ws/crossplane.yaml", testMetav1alpha1, os.ModePerm)
					_ = afero.WriteFile(fs, "/ws/crds/crd.yaml", testCRD, os.ModePerm)
					_ = afero.WriteFile(fs, "/ws/examples/ec2/instance.yaml", testEx1, os.ModePerm)
					_ = afero.WriteFile(fs, "/ws/examples/ec2/internetgateway.yaml", testEx2, os.ModePerm)
					_ = afero.WriteFile(fs, "/ws/examples/ecr/repository.yaml", testEx3, os.ModePerm)
					_ = afero.WriteFile(fs, "/ws/examples/provider.yaml", testEx4, os.ModePerm)
					return fs
				},
			},
			want: want{
				pkgExists: true,
				exExists:  true,
				labels: []string{
					PackageAnnotation,
					ExamplesAnnotation,
				},
			},
		},
		"SuccessExamplesAtCustomDir": {
			args: args{
				rootDir:     "/ws",
				examplesDir: "/other_directory/examples",
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					_ = fs.Mkdir("/ws", os.ModePerm)
					_ = fs.Mkdir("/other_directory", os.ModePerm)
					_ = afero.WriteFile(fs, "/ws/crossplane.yaml", testMetav1alpha1, os.ModePerm)
					_ = afero.WriteFile(fs, "/ws/crds/crd.yaml", testCRD, os.ModePerm)
					_ = afero.WriteFile(fs, "/other_directory/examples/ec2/instance.yaml", testEx1, os.ModePerm)
					_ = afero.WriteFile(fs, "/other_directory/examples/ec2/internetgateway.yaml", testEx2, os.ModePerm)
					_ = afero.WriteFile(fs, "/other_directory/examples/ecr/repository.yaml", testEx3, os.ModePerm)
					_ = afero.WriteFile(fs, "/other_directory/examples/provider.yaml", testEx4, os.ModePerm)
					return fs
				},
			},
			want: want{
				pkgExists: true,
				exExists:  true,
				labels: []string{
					PackageAnnotation,
					ExamplesAnnotation,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			pkgBe := parser.NewFsBackend(
				tc.args.fs(),
				parser.FsDir(tc.args.rootDir),
				parser.FsFilters([]parser.FilterFn{
					parser.SkipDirs(),
					parser.SkipNotYAML(),
					parser.SkipEmpty(),
					SkipContains("examples/"), // don't try to parse the examples in the package
				}...),
			)
			pkgEx := parser.NewFsBackend(
				tc.args.fs(),
				parser.FsDir(tc.args.examplesDir),
				parser.FsFilters(defaultFilters...),
			)

			builder := New(pkgBe, nil, pkgEx, nil, pkgp, examples.New())

			img, _, err := builder.Build(context.TODO())

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nBuildExamples(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			// validate the xpkg img has the correct annotations, etc
			contents, err := readImg(img)
			// sort the contents slice for test comparison
			sort.Strings(contents.labels)

			if diff := cmp.Diff(tc.want.pkgExists, len(contents.pkgBytes) != 0); diff != "" {
				t.Errorf("\n%s\nBuildExamples(...): -want err, +got err:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.exExists, len(contents.exBytes) != 0); diff != "" {
				t.Errorf("\n%s\nBuildExamples(...): -want err, +got err:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.labels, contents.labels, cmpopts.SortSlices(func(i, j int) bool {
				return contents.labels[i] < contents.labels[j]
			})); diff != "" {
				t.Errorf("\n%s\nBuildExamples(...): -want err, +got err:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(nil, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nBuildExamples(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestBuildAuth(t *testing.T) {
	pkgp, _ := yaml.New()

	type withFsFn func() afero.Fs

	type args struct {
		rootDir string
		// The auth parser backend is constructed then passed in during
		// initialization. We mimic that behavior here instead of strictly
		// relying on the filesystem contents.
		authBE parser.Backend
		fs     withFsFn
	}
	type want struct {
		pkgExists  bool
		authExists bool
		err        error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SuccessNoAuthV1alpha1Provider": {
			args: args{
				rootDir: "/ws",
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					_ = fs.Mkdir("/ws", os.ModePerm)
					_ = fs.Mkdir("/ws/crds", os.ModePerm)
					_ = afero.WriteFile(fs, "/ws/crossplane.yaml", testMetav1alpha1, os.ModePerm)
					_ = afero.WriteFile(fs, "/ws/crds/crd.yaml", testCRD, os.ModePerm)
					return fs
				},
			},
			want: want{
				pkgExists: true,
			},
		},
		"SuccessNoAuthV1Provider": {
			args: args{
				rootDir: "/ws",
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					_ = fs.Mkdir("/ws", os.ModePerm)
					_ = fs.Mkdir("/ws/crds", os.ModePerm)
					_ = afero.WriteFile(fs, "/ws/crossplane.yaml", testMetav1, os.ModePerm)
					_ = afero.WriteFile(fs, "/ws/crds/crd.yaml", testCRD, os.ModePerm)
					return fs
				},
			},
			want: want{
				pkgExists: true,
			},
		},
		"SuccessAuthV1Alpha1Provider": {
			args: args{
				rootDir: "/ws",
				authBE:  parser.NewEchoBackend(string(testAuth)),
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					_ = fs.Mkdir("/ws", os.ModePerm)
					_ = fs.Mkdir("/ws/crds", os.ModePerm)
					_ = afero.WriteFile(fs, "/ws/crossplane.yaml", testMetav1alpha1WAuth, os.ModePerm)
					_ = afero.WriteFile(fs, "/ws/crds/providerconfig.yaml", testPC, os.ModePerm)
					_ = afero.WriteFile(fs, "/ws/crds/crd.yaml", testCRD, os.ModePerm)
					return fs
				},
			},
			want: want{
				pkgExists:  true,
				authExists: true,
			},
		},
		"SuccessAuthV1Provider": {
			args: args{
				rootDir: "/ws",
				authBE:  parser.NewEchoBackend(string(testAuth)),
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					_ = fs.Mkdir("/ws", os.ModePerm)
					_ = fs.Mkdir("/ws/crds", os.ModePerm)
					_ = afero.WriteFile(fs, "/ws/crossplane.yaml", testMetav1WAuth, os.ModePerm)
					_ = afero.WriteFile(fs, "/ws/crds/providerconfig.yaml", testPC, os.ModePerm)
					_ = afero.WriteFile(fs, "/ws/crds/crd.yaml", testCRD, os.ModePerm)
					return fs
				},
			},
			want: want{
				pkgExists:  true,
				authExists: true,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			pkgBe := parser.NewFsBackend(
				tc.args.fs(),
				parser.FsDir(tc.args.rootDir),
				parser.FsFilters([]parser.FilterFn{
					parser.SkipDirs(),
					parser.SkipNotYAML(),
					parser.SkipEmpty(),
					SkipContains("examples/"), // don't try to parse the examples in the package
				}...),
			)

			pkgEx := parser.NewFsBackend(
				tc.args.fs(),
				parser.FsFilters(defaultFilters...),
			)

			builder := New(pkgBe, tc.args.authBE, pkgEx, nil, pkgp, examples.New())

			img, _, err := builder.Build(context.TODO())

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nBuildAuth(...): -want err, +got err:\n%s", tc.reason, diff)
			}
			// validate the xpkg img has the correct annotations, etc
			contents, err := readImg(img)
			// sort the contents slice for test comparison
			sort.Strings(contents.labels)

			if diff := cmp.Diff(tc.want.pkgExists, len(contents.pkgBytes) != 0); diff != "" {
				t.Errorf("\n%s\nBuildAuth(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.authExists, contents.includesAuth); diff != "" {
				t.Errorf("\n%s\nBuildAuth(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(nil, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nBuildAuth(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestBuildHelm(t *testing.T) {
	pkgp, _ := yaml.New()
	errBoom := errors.New("boom")

	type withFsFn func() afero.Fs

	type args struct {
		rootDir string
		// The helm parser backend is constructed then passed in during
		// initialization. We mimic that behavior here instead of strictly
		// relying on the filesystem contents.
		helmBE parser.Backend
		fs     withFsFn
	}
	type want struct {
		pkgExists  bool
		helmExists bool
		labels     []string
		err        error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SuccessNoHelmChartNonController": {
			reason: "Non-controller packages should not require a Helm chart",
			args: args{
				rootDir: "/ws",
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					_ = fs.Mkdir("/ws", os.ModePerm)
					_ = fs.Mkdir("/ws/crds", os.ModePerm)
					_ = afero.WriteFile(fs, "/ws/crossplane.yaml", testMetav1alpha1, os.ModePerm)
					_ = afero.WriteFile(fs, "/ws/crds/crd.yaml", testCRD, os.ModePerm)
					return fs
				},
			},
			want: want{
				pkgExists: true,
				labels: []string{
					PackageAnnotation,
				},
			},
		},
		"SuccessWithHelmChart": {
			reason: "Controller packages with a Helm chart should succeed",
			args: args{
				rootDir: "/ws",
				helmBE:  parser.NewEchoBackend(string(testHelmChart)),
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					_ = fs.Mkdir("/ws", os.ModePerm)
					_ = fs.Mkdir("/ws/crds", os.ModePerm)
					// Use a controller meta file for this test
					_ = afero.WriteFile(fs, "/ws/crossplane.yaml", testController, os.ModePerm)
					_ = afero.WriteFile(fs, "/ws/crds/crd.yaml", testCRD, os.ModePerm)
					return fs
				},
			},
			want: want{
				pkgExists:  true,
				helmExists: true,
				labels: []string{
					PackageAnnotation,
					HelmChartAnnotation,
				},
			},
		},
		"ErrControllerNoHelmChart": {
			reason: "Controller packages without a Helm chart should fail",
			args: args{
				rootDir: "/ws",
				helmBE:  nil, // No Helm chart provided
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					_ = fs.Mkdir("/ws", os.ModePerm)
					_ = fs.Mkdir("/ws/crds", os.ModePerm)
					_ = afero.WriteFile(fs, "/ws/crossplane.yaml", testController, os.ModePerm)
					_ = afero.WriteFile(fs, "/ws/crds/crd.yaml", testCRD, os.ModePerm)
					return fs
				},
			},
			want: want{
				err: errors.New(errControllerNoHelm),
			},
		},
		"SuccessWithHelmChartButNotController": {
			reason: "Non-controller packages with a Helm chart should succeed but not include the chart",
			args: args{
				rootDir: "/ws",
				helmBE:  parser.NewEchoBackend(string(testHelmChart)),
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					_ = fs.Mkdir("/ws", os.ModePerm)
					_ = fs.Mkdir("/ws/crds", os.ModePerm)
					// Use a non-controller meta file for this test
					_ = afero.WriteFile(fs, "/ws/crossplane.yaml", testMetav1alpha1, os.ModePerm)
					_ = afero.WriteFile(fs, "/ws/crds/crd.yaml", testCRD, os.ModePerm)
					return fs
				},
			},
			want: want{
				pkgExists: true,
				labels: []string{
					PackageAnnotation,
				},
			},
		},
		"ErrInitHelmBackend": {
			reason: "Should return an error if we fail to initialize helm backend for a controller package",
			args: args{
				rootDir: "/ws",
				helmBE: &MockBackend{
					MockInit: NewMockInitFn(nil, errBoom),
				},
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					_ = fs.Mkdir("/ws", os.ModePerm)
					_ = fs.Mkdir("/ws/crds", os.ModePerm)
					_ = afero.WriteFile(fs, "/ws/crossplane.yaml", testController, os.ModePerm)
					_ = afero.WriteFile(fs, "/ws/crds/crd.yaml", testCRD, os.ModePerm)
					return fs
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errInitHelmBackend),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			pkgBe := parser.NewFsBackend(
				tc.args.fs(),
				parser.FsDir(tc.args.rootDir),
				parser.FsFilters([]parser.FilterFn{
					parser.SkipDirs(),
					parser.SkipNotYAML(),
					parser.SkipEmpty(),
					SkipContains("examples/"), // don't try to parse the examples in the package
				}...),
			)

			pkgEx := parser.NewFsBackend(
				tc.args.fs(),
				parser.FsFilters(defaultFilters...),
			)

			builder := New(pkgBe, nil, pkgEx, tc.args.helmBE, pkgp, examples.New())

			img, _, err := builder.Build(context.TODO())

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nBuildHelm(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			if err != nil {
				return
			}

			// validate the xpkg img has the correct annotations, etc
			contents, err := readImg(img)
			// sort the contents slice for test comparison
			sort.Strings(contents.labels)

			if diff := cmp.Diff(tc.want.pkgExists, len(contents.pkgBytes) != 0); diff != "" {
				t.Errorf("\n%s\nBuildHelm(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.helmExists, len(contents.helmBytes) != 0); diff != "" {
				t.Errorf("\n%s\nBuildHelm(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.labels, contents.labels, cmpopts.SortSlices(func(i, j int) bool {
				return contents.labels[i] < contents.labels[j]
			})); diff != "" {
				t.Errorf("\n%s\nBuildHelm(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(nil, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nBuildHelm(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

type xpkgContents struct {
	labels       []string
	pkgBytes     []byte
	exBytes      []byte
	helmBytes    []byte
	includesAuth bool
}

func readImg(i v1.Image) (xpkgContents, error) {
	contents := xpkgContents{
		labels: make([]string, 0),
	}

	reader := mutate.Extract(i)
	fs := tarfs.New(tar.NewReader(reader))
	pkgYaml, err := fs.Open(StreamFile)
	if err != nil {
		return contents, err
	}

	pkgBytes, err := io.ReadAll(pkgYaml)
	if err != nil {
		return contents, err
	}
	contents.pkgBytes = pkgBytes
	ps := string(pkgBytes)

	// This is pretty unfortunate. Unless we build out steps to re-parse the
	// package from the image (i.e. the system under test) we're left
	// performing string parsing. For now we choose part of the auth spec,
	// specifically the version and date used in auth yamls.
	if strings.Contains(ps, AuthObjectAnno) {
		contents.includesAuth = strings.Contains(ps, "version: \"2023-06-23\"")
	}

	exYaml, err := fs.Open(XpkgExamplesFile)
	if err != nil && !os.IsNotExist(err) {
		return contents, err
	}

	if exYaml != nil {
		exBytes, err := io.ReadAll(exYaml)
		if err != nil {
			return contents, err
		}
		contents.exBytes = exBytes
	}

	helmChart, err := fs.Open(XpkgHelmChartFile)
	if err != nil && !os.IsNotExist(err) {
		return contents, err
	}

	if helmChart != nil {
		helmBytes, err := io.ReadAll(helmChart)
		if err != nil {
			return contents, err
		}
		contents.helmBytes = helmBytes
	}

	labels, err := allLabels(i)
	if err != nil {
		return contents, err
	}

	contents.labels = labels

	return contents, nil
}

func allLabels(i partial.WithConfigFile) ([]string, error) {
	labels := []string{}

	cfgFile, err := i.ConfigFile()
	if err != nil {
		return labels, err
	}

	cfg := cfgFile.Config

	for _, label := range cfg.Labels {
		labels = append(labels, label)
	}

	return labels, nil
}
