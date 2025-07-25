// Copyright 2025 Upbound Inc.
// All rights reserved
//revive:disable:error-strings

package xpkg

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/spf13/afero"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	xpmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	xpmetav1alpha1 "github.com/crossplane/crossplane/apis/pkg/meta/v1alpha1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"

	"github.com/upbound/up/internal/xpkg"
)

const (
	testdata = "../../../testdata"

	testProviderConfigsCRD      = "providerconfigs.helm.crossplane.io.yaml"
	testProviderConfigUsagesCRD = "providerconfigusages.helm.crossplane.io.yaml"

	// "go get" does require filenames to be cross-platform compatible.
	// ":" is an invalid character on Windows.
	testDigestFileOSPath = "sha256_295bcd0e6dc396cf0f5ef638c8a7610a571ff2dcef3aa0447398f25b5a0eafc7"
	testDigestFile       = "sha256:295bcd0e6dc396cf0f5ef638c8a7610a571ff2dcef3aa0447398f25b5a0eafc7"
	testPackageJSONFile2 = "package2.ndjson"
)

var testProviderPkgYaml = filepath.Join(testdata, "provider_package.yaml")

func TestFromImage(t *testing.T) {
	type args struct {
		img xpkg.Image
	}

	type want struct {
		pkg        *ParsedPackage
		numObjects int
		err        error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Success": {
			reason: "Should return a ParsedPackage and no error.",
			args: args{
				img: xpkg.Image{
					Meta: xpkg.ImageMeta{
						Registry: "index.docker.io",
						Repo:     "crossplane/provider-aws",
						Version:  "v0.20.0",
						Digest:   "sha256:03d3217f3b69432a9d3313d318c1ab6980fff5245134615dd8e32357ce3851c9",
					},
					Image: newPackageImage(testProviderPkgYaml),
				},
			},
			want: want{
				pkg: &ParsedPackage{
					SHA: digest(newPackageImage(testProviderPkgYaml)),
					Deps: []v1beta1.Dependency{
						{
							Package:     "crossplane/provider-gcp",
							Type:        ptr.To(v1beta1.ProviderPackageType),
							Constraints: "v0.18.0",
						},
					},
					MetaObj: &xpmetav1alpha1.Provider{
						TypeMeta: apimetav1.TypeMeta{
							APIVersion: "meta.pkg.crossplane.io/v1alpha1",
							Kind:       "Provider",
						},
						ObjectMeta: apimetav1.ObjectMeta{
							Name: "provider-aws",
						},
						Spec: xpmetav1alpha1.ProviderSpec{
							Controller: xpmetav1alpha1.ControllerSpec{
								Image: ptr.To("crossplane/provider-aws-controller:v0.20.0"),
							},
							MetaSpec: xpmetav1alpha1.MetaSpec{
								DependsOn: []xpmetav1alpha1.Dependency{
									{
										Provider: ptr.To("crossplane/provider-gcp"),
										Version:  "v0.18.0",
									},
								},
							},
						},
					},
					PType: v1beta1.ProviderPackageType,
					Reg:   "index.docker.io",
					Ver:   "v0.20.0",
				},
				numObjects: 2,
			},
		},
		"ErrInvalidPackageImage": {
			reason: "Should return an error if package image is invalid.",
			args: args{
				img: xpkg.Image{
					Image: empty.Image,
				},
			},
			want: want{
				err: errors.Wrapf(errors.New("unknown blob :"), "failed to find the package layer"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			pkgres, _ := NewMarshaler()

			pkg, err := pkgres.FromImage(tc.args.img)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nFromImage(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			if err == nil {
				if diff := cmp.Diff(tc.want.pkg.Digest(), pkg.Digest()); diff != "" {
					t.Errorf("\n%s\nFromImage(...): -want err, +got err:\n%s", tc.reason, diff)
				}

				if diff := cmp.Diff(tc.want.pkg.Dependencies(), pkg.Dependencies()); diff != "" {
					t.Errorf("\n%s\nFromImage(...): -want err, +got err:\n%s", tc.reason, diff)
				}

				if diff := cmp.Diff(tc.want.pkg.Meta(), pkg.Meta()); diff != "" {
					t.Errorf("\n%s\nFromImage(...): -want err, +got err:\n%s", tc.reason, diff)
				}

				if diff := cmp.Diff(tc.want.numObjects, len(pkg.Objects())); diff != "" {
					t.Errorf("\n%s\nFromImage(...): -want err, +got err:\n%s", tc.reason, diff)
				}

				if diff := cmp.Diff(tc.want.pkg.Registry(), pkg.Registry()); diff != "" {
					t.Errorf("\n%s\nFromImage(...): -want err, +got err:\n%s", tc.reason, diff)
				}

				if diff := cmp.Diff(tc.want.pkg.Type(), pkg.Type()); diff != "" {
					t.Errorf("\n%s\nFromImage(...): -want err, +got err:\n%s", tc.reason, diff)
				}

				if diff := cmp.Diff(tc.want.pkg.Version(), pkg.Version()); diff != "" {
					t.Errorf("\n%s\nFromImage(...): -want err, +got err:\n%s", tc.reason, diff)
				}
			}
		})
	}
}

func TestFromDir(t *testing.T) {
	inmemfs := afero.NewMemMapFs()
	testdatafs := afero.NewOsFs()
	path1 := "cache/index.docker.io/crossplane/provider-helm@v0.9.0"
	path2 := "cache/registry.upbound.io/upbound/platform-ref-aws@v0.9.0"

	_ = inmemfs.MkdirAll(path1, os.ModePerm)
	_ = inmemfs.MkdirAll(path2, os.ModePerm)

	// write files to the above path
	json, _ := testdatafs.Open(filepath.Join(testdata, xpkg.JSONStreamFile))
	defer json.Close()
	meta, _ := testdatafs.Open(filepath.Join(testdata, xpkg.MetaFile))
	defer meta.Close()
	crd1, _ := testdatafs.Open(filepath.Join(testdata, testProviderConfigsCRD))
	defer crd1.Close()
	crd2, _ := testdatafs.Open(filepath.Join(testdata, testProviderConfigUsagesCRD))
	defer crd2.Close()
	sha, _ := testdatafs.Open(filepath.Join(testdata, testDigestFileOSPath))
	defer sha.Close()

	json2, _ := testdatafs.Open(filepath.Join(testdata, testPackageJSONFile2))
	defer json.Close()
	meta2, _ := testdatafs.Open(filepath.Join(testdata, xpkg.MetaFile))
	defer meta.Close()
	crd12, _ := testdatafs.Open(filepath.Join(testdata, testProviderConfigsCRD))
	defer crd1.Close()
	crd22, _ := testdatafs.Open(filepath.Join(testdata, testProviderConfigUsagesCRD))
	defer crd2.Close()
	sha2, _ := testdatafs.Open(filepath.Join(testdata, testDigestFileOSPath))
	defer sha.Close()

	targetPkgJSON, _ := inmemfs.Create(filepath.Join(path1, xpkg.JSONStreamFile))
	targetMeta, _ := inmemfs.Create(filepath.Join(path1, xpkg.MetaFile))
	targetCRD1, _ := inmemfs.Create(filepath.Join(path1, testProviderConfigsCRD))
	targetCRD2, _ := inmemfs.Create(filepath.Join(path1, testProviderConfigUsagesCRD))
	targetSHA, _ := inmemfs.Create(filepath.Join(path1, testDigestFile))

	io.Copy(targetPkgJSON, json)
	io.Copy(targetMeta, meta)
	io.Copy(targetCRD1, crd1)
	io.Copy(targetCRD2, crd2)
	io.Copy(targetSHA, sha)

	targetPkgJSON2, _ := inmemfs.Create(filepath.Join(path2, xpkg.JSONStreamFile))
	targetMeta2, _ := inmemfs.Create(filepath.Join(path2, xpkg.MetaFile))
	targetCRD12, _ := inmemfs.Create(filepath.Join(path2, testProviderConfigsCRD))
	targetCRD22, _ := inmemfs.Create(filepath.Join(path2, testProviderConfigUsagesCRD))
	targetSHA2, _ := inmemfs.Create(filepath.Join(path2, testDigestFile))

	io.Copy(targetPkgJSON2, json2)
	io.Copy(targetMeta2, meta2)
	io.Copy(targetCRD12, crd12)
	io.Copy(targetCRD22, crd22)
	io.Copy(targetSHA2, sha2)

	type args struct {
		path string
	}
	type want struct {
		pkg        *ParsedPackage
		numObjects int
		err        error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SuccessDockerHubPackage": {
			reason: "Should return a ParsedPackage and no error.",
			args: args{
				path: path1,
			},
			want: want{
				pkg: &ParsedPackage{
					SHA: testDigestFile,
					Deps: []v1beta1.Dependency{
						{
							Package:     "crossplane/provider-aws",
							Type:        ptr.To(v1beta1.ProviderPackageType),
							Constraints: "v0.20.0",
						},
					},
					MetaObj: &xpmetav1alpha1.Provider{
						TypeMeta: apimetav1.TypeMeta{
							APIVersion: "meta.pkg.crossplane.io/v1alpha1",
							Kind:       "Provider",
						},
						ObjectMeta: apimetav1.ObjectMeta{
							Name: "provider-helm",
						},
						Spec: xpmetav1alpha1.ProviderSpec{
							Controller: xpmetav1alpha1.ControllerSpec{
								Image: ptr.To("crossplane/provider-helm-controller:v0.9.0"),
							},
							MetaSpec: xpmetav1alpha1.MetaSpec{
								DependsOn: []xpmetav1alpha1.Dependency{
									{
										Provider: ptr.To("crossplane/provider-aws"),
										Version:  "v0.20.0",
									},
								},
							},
						},
					},
					DepName: "crossplane/provider-helm",
					PType:   v1beta1.ProviderPackageType,
					Reg:     "index.docker.io",
					Ver:     "v0.9.0",
				},
				numObjects: 2,
			},
		},
		"SuccessNonDockerHubPackage": {
			reason: "Should return a ParsedPackage and no error.",
			args: args{
				path: path2,
			},
			want: want{
				pkg: &ParsedPackage{
					SHA: testDigestFile,
					Deps: []v1beta1.Dependency{
						{
							Package:     "crossplane/provider-aws",
							Type:        ptr.To(v1beta1.ProviderPackageType),
							Constraints: "v0.20.0",
						},
					},
					MetaObj: &xpmetav1alpha1.Provider{
						TypeMeta: apimetav1.TypeMeta{
							APIVersion: "meta.pkg.crossplane.io/v1alpha1",
							Kind:       "Provider",
						},
						ObjectMeta: apimetav1.ObjectMeta{
							Name: "provider-helm",
						},
						Spec: xpmetav1alpha1.ProviderSpec{
							Controller: xpmetav1alpha1.ControllerSpec{
								Image: ptr.To("crossplane/provider-helm-controller:v0.9.0"),
							},
							MetaSpec: xpmetav1alpha1.MetaSpec{
								DependsOn: []xpmetav1alpha1.Dependency{
									{
										Provider: ptr.To("crossplane/provider-aws"),
										Version:  "v0.20.0",
									},
								},
							},
						},
					},
					DepName: "registry.upbound.io/crossplane/provider-helm",
					PType:   v1beta1.ProviderPackageType,
					Reg:     "registry.upbound.io",
					Ver:     "v0.9.0",
				},
				numObjects: 2,
			},
		},
		"ErrInvalidPackagePath": {
			reason: "Should return an error if path is invalid.",
			args: args{
				path: "/notapackage",
			},
			want: want{
				err: errors.New(errInvalidPath),
			},
		},
		"ErrInvalidPackage": {
			reason: "Should return an error if package is invalid at provided path.",
			args: args{
				path: "/notapackage@v0.0.0",
			},
			want: want{
				err: &os.PathError{Op: "open", Path: "/notapackage@v0.0.0/package.ndjson", Err: os.ErrNotExist},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			pkgres, _ := NewMarshaler()

			pkg, err := pkgres.FromDir(inmemfs, tc.args.path)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nFromDir(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			if err == nil {
				if diff := cmp.Diff(tc.want.pkg.Digest(), pkg.Digest()); diff != "" {
					t.Errorf("\n%s\nFromDir(...): -want err, +got err:\n%s", tc.reason, diff)
				}

				if diff := cmp.Diff(tc.want.pkg.Dependencies(), pkg.Dependencies()); diff != "" {
					t.Errorf("\n%s\nFromDir(...): -want err, +got err:\n%s", tc.reason, diff)
				}

				if diff := cmp.Diff(tc.want.pkg.Meta(), pkg.Meta()); diff != "" {
					t.Errorf("\n%s\nFromDir(...): -want err, +got err:\n%s", tc.reason, diff)
				}

				if diff := cmp.Diff(tc.want.pkg.Name(), pkg.Name()); diff != "" {
					t.Errorf("\n%s\nFromDir(...): -want err, +got err:\n%s", tc.reason, diff)
				}

				if diff := cmp.Diff(tc.want.numObjects, len(pkg.Objects())); diff != "" {
					t.Errorf("\n%s\nFromDir(...): -want err, +got err:\n%s", tc.reason, diff)
				}

				if diff := cmp.Diff(tc.want.pkg.Registry(), pkg.Registry()); diff != "" {
					t.Errorf("\n%s\nFromDir(...): -want err, +got err:\n%s", tc.reason, diff)
				}

				if diff := cmp.Diff(tc.want.pkg.Type(), pkg.Type()); diff != "" {
					t.Errorf("\n%s\nFromDir(...): -want err, +got err:\n%s", tc.reason, diff)
				}

				if diff := cmp.Diff(tc.want.pkg.Version(), pkg.Version()); diff != "" {
					t.Errorf("\n%s\nFromDir(...): -want err, +got err:\n%s", tc.reason, diff)
				}
			}
		})
	}
}

func newPackageImage(path string) v1.Image {
	pack, _ := os.Open(path)

	info, _ := pack.Stat()

	buf := new(bytes.Buffer)

	tw := tar.NewWriter(buf)
	hdr := &tar.Header{
		Name: xpkg.StreamFile,
		Mode: int64(xpkg.StreamFileMode),
		Size: info.Size(),
	}
	_ = tw.WriteHeader(hdr)
	_, _ = io.Copy(tw, pack)
	_ = tw.Close()
	packLayer, _ := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		// NOTE(hasheddan): we must construct a new reader each time as we
		// ingest packImg in multiple tests below.
		return io.NopCloser(bytes.NewReader(buf.Bytes())), nil
	})

	packLayerDigest, _ := packLayer.Digest()
	packImg, _ := mutate.AppendLayers(empty.Image, packLayer)

	cfg, _ := packImg.ConfigFile()
	if cfg.Config.Labels == nil {
		cfg.Config.Labels = map[string]string{}
	}
	labelKey := xpkg.Label(packLayerDigest.String())
	cfg.Config.Labels[labelKey] = xpkg.PackageAnnotation
	packImg, _ = mutate.Config(packImg, cfg.Config)
	packImg, _ = xpkg.AnnotateImage(packImg)
	return packImg
}

func digest(i v1.Image) string {
	h, _ := i.Digest()
	return h.String()
}

type MetaSpec struct {
	DependsOn []xpmetav1.Dependency
}

type Spec struct {
	MetaSpec MetaSpec
}

type Metadata struct {
	Spec Spec
}

func TestProcessDependsOn(t *testing.T) {
	configKind := "Configuration"
	functionKind := "Function"
	providerKind := "Provider"
	packageValue := "test-package"

	cases := map[string]struct {
		input Metadata
		want  Metadata
		error error
	}{
		"ValidConfiguration": {
			input: Metadata{
				Spec: Spec{
					MetaSpec: MetaSpec{
						DependsOn: []xpmetav1.Dependency{
							{
								Kind:    &configKind,
								Package: &packageValue,
							},
						},
					},
				},
			},
			want: Metadata{
				Spec: Spec{
					MetaSpec: MetaSpec{
						DependsOn: []xpmetav1.Dependency{
							{
								Kind:          &configKind,
								Package:       &packageValue,
								Configuration: &packageValue,
							},
						},
					},
				},
			},
			error: nil,
		},
		"ValidFunction": {
			input: Metadata{
				Spec: Spec{
					MetaSpec: MetaSpec{
						DependsOn: []xpmetav1.Dependency{
							{
								Kind:    &functionKind,
								Package: &packageValue,
							},
						},
					},
				},
			},
			want: Metadata{
				Spec: Spec{
					MetaSpec: MetaSpec{
						DependsOn: []xpmetav1.Dependency{
							{
								Kind:     &functionKind,
								Package:  &packageValue,
								Function: &packageValue,
							},
						},
					},
				},
			},
			error: nil,
		},
		"ValidProvider": {
			input: Metadata{
				Spec: Spec{
					MetaSpec: MetaSpec{
						DependsOn: []xpmetav1.Dependency{
							{
								Kind:    &providerKind,
								Package: &packageValue,
							},
						},
					},
				},
			},
			want: Metadata{
				Spec: Spec{
					MetaSpec: MetaSpec{
						DependsOn: []xpmetav1.Dependency{
							{
								Kind:     &providerKind,
								Package:  &packageValue,
								Provider: &packageValue,
							},
						},
					},
				},
			},
			error: nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			input := tc.input
			err := processDependsOn(&input)

			if diff := cmp.Diff(tc.want, input); diff != "" {
				t.Errorf("processDependsOn() mismatch (-want +got):\n%s", diff)
			}

			if (err != nil) != (tc.error != nil) || (err != nil && err.Error() != tc.error.Error()) {
				t.Errorf("Expected error %v, got %v", tc.error, err)
			}
		})
	}
}
