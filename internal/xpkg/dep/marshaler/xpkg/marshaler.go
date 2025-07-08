// Copyright 2025 Upbound Inc.
// All rights reserved

// Package xpkg contains function for marshal xpkg packages.
package xpkg

import (
	"archive/tar"
	"context"
	"io"
	"path/filepath"
	"strings"

	cv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/spf13/afero"
	"github.com/spf13/afero/tarfs"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/parser"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	xpmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	xpmetav1beta1 "github.com/crossplane/crossplane/apis/pkg/meta/v1beta1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"

	upboundpkgmetav1alpha1 "github.com/upbound/up-sdk-go/apis/pkg/meta/v1alpha1"
	"github.com/upbound/up/internal/xpkg"
	"github.com/upbound/up/internal/xpkg/parser/linter"
	"github.com/upbound/up/internal/xpkg/parser/ndjson"
	"github.com/upbound/up/internal/xpkg/parser/yaml"
	"github.com/upbound/up/internal/xpkg/scheme"
)

const (
	errFailedToParsePkgYaml         = "failed to parse package yaml"
	errLintPackage                  = "failed to lint package"
	errOpenPackageStream            = "failed to open package stream file"
	errConvertXRDs                  = "failed to convert XRD to CRD"
	errFailedToConvertMetaToPackage = "failed to convert meta to package"
	errInvalidPath                  = "invalid path provided for package lookup"
	errNotExactlyOneMeta            = "not exactly one package meta type"
	maxFileSize                     = 1024 * 1024 * 1024
)

// Marshaler represents a xpkg Marshaler.
type Marshaler struct {
	yp parser.Parser
	jp JSONPackageParser
}

// NewMarshaler returns a new Marshaler.
func NewMarshaler(opts ...MarshalerOption) (*Marshaler, error) {
	r := &Marshaler{}
	yp, err := yaml.New()
	if err != nil {
		return nil, err
	}

	jp, err := ndjson.New()
	if err != nil {
		return nil, err
	}

	r.yp = yp
	r.jp = jp

	for _, o := range opts {
		o(r)
	}

	return r, nil
}

// MarshalerOption modifies the xpkg Marshaler.
type MarshalerOption func(*Marshaler)

// WithYamlParser modifies the Marshaler by setting the supplied PackageParser as
// the Resolver's parser.
func WithYamlParser(p parser.Parser) MarshalerOption {
	return func(r *Marshaler) {
		r.yp = p
	}
}

// WithJSONParser modifies the Marshaler by setting the supplied PackageParser as
// the Resolver's parser.
func WithJSONParser(p JSONPackageParser) MarshalerOption {
	return func(r *Marshaler) {
		r.jp = p
	}
}

// FromImage takes a xpkg.Image and returns a ParsedPackage for consumption by
// upstream callers.
func (r *Marshaler) FromImage(i xpkg.Image) (*ParsedPackage, error) { //nolint:gocyclo // layer handler
	manifest, err := i.Image.Manifest()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get image manifest")
	}

	var packageLayerDigest cv1.Hash

	for _, l := range manifest.Layers {
		if val, ok := l.Annotations[xpkg.AnnotationKey]; ok && val == xpkg.PackageAnnotation {
			packageLayerDigest = l.Digest
		}
	}

	if packageLayerDigest.String() == "" {
		return nil, errors.New("package layer with specified annotation not found")
	}

	packageLayer, err := i.Image.LayerByDigest(packageLayerDigest)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find the package layer")
	}

	reader, err := packageLayer.Uncompressed()
	if err != nil {
		return nil, errors.Wrap(err, "failed to extract package layer")
	}

	fs := tarfs.New(tar.NewReader(reader))
	pkgYaml, err := fs.Open(xpkg.StreamFile)
	if err != nil {
		return nil, errors.Wrap(err, errOpenPackageStream)
	}

	pkg, err := r.parseYaml(pkgYaml)
	if err != nil {
		return nil, err
	}

	pkg = applyImageMeta(i.Meta, pkg)
	return finalizePkg(pkg)
}

// FromDir takes an afero.Fs and a path to a directory and returns a
// ParsedPackage based on the directories contents for consumption by upstream
// callers.
func (r *Marshaler) FromDir(fs afero.Fs, path string) (*ParsedPackage, error) {
	parts := strings.Split(path, "@")
	if len(parts) != 2 {
		return nil, errors.New(errInvalidPath)
	}

	pkgJSON, err := fs.Open(filepath.Join(path, xpkg.JSONStreamFile))
	if err != nil {
		return nil, err
	}

	pkg, err := r.parseNDJSON(pkgJSON)
	if err != nil {
		return nil, err
	}

	return finalizePkg(pkg)
}

// parseYaml parses the.
func (r *Marshaler) parseYaml(reader io.ReadCloser) (*ParsedPackage, error) {
	pkg, err := r.yp.Parse(context.Background(), reader)
	if err != nil {
		return nil, errors.Wrap(err, errFailedToParsePkgYaml)
	}
	return processPackage(pkg)
}

func processPackage(pkg linter.Package) (*ParsedPackage, error) {
	metas := pkg.GetMeta()
	if len(metas) != 1 {
		return nil, errors.New(errNotExactlyOneMeta)
	}

	meta := metas[0]

	var linter linter.Linter
	gvk := meta.GetObjectKind().GroupVersionKind()
	switch gvk.Kind {
	case xpmetav1.ConfigurationKind:
		linter = xpkg.NewConfigurationLinter()
	case xpmetav1.ProviderKind:
		linter = xpkg.NewProviderLinter()
	case xpmetav1beta1.FunctionKind:
		linter = xpkg.NewFunctionLinter()
	case upboundpkgmetav1alpha1.ControllerKind:
		linter = xpkg.NewControllerLinter()
	}
	if err := linter.Lint(pkg); err != nil {
		return nil, errors.Wrap(err, errLintPackage)
	}

	return &ParsedPackage{
		MetaObj: meta,
		Objs:    pkg.GetObjects(),
		// strip out meta.
		APIVersion: gvk.GroupVersion().String()[5:],
		Kind:       gvk.Kind,
	}, nil
}

func (r *Marshaler) parseNDJSON(reader io.ReadCloser) (*ParsedPackage, error) {
	pkg, err := r.jp.Parse(context.Background(), reader)
	if err != nil {
		return nil, errors.Wrap(err, errFailedToParsePkgYaml)
	}

	metas := pkg.GetMeta()
	if len(metas) != 1 {
		return nil, errors.New(errNotExactlyOneMeta)
	}

	meta := metas[0]

	// Check if the meta kind is ConfigurationKind
	if meta.GetObjectKind().GroupVersionKind().Kind == xpmetav1.ConfigurationKind {
		filteredObjects := []runtime.Object{}
		for _, obj := range pkg.GetObjects() {
			// Only include objects of type CompositeResourceDefinition or Composition
			if _, isXRD := obj.(*v1.CompositeResourceDefinition); isXRD {
				filteredObjects = append(filteredObjects, obj)
			} else if _, isComposition := obj.(*v1.Composition); isComposition {
				filteredObjects = append(filteredObjects, obj)
			}
		}
		// Replace pkg.objects with the filtered list
		pkg.SetObjects(filteredObjects)
	}

	p, err := processPackage(pkg)
	if err != nil {
		return nil, err
	}

	return applyImageMeta(pkg.GetImageMeta(), p), nil
}

func applyImageMeta(m xpkg.ImageMeta, pkg *ParsedPackage) *ParsedPackage {
	pkg.DepName = m.Repo
	pkg.Reg = m.Registry
	pkg.SHA = m.Digest
	pkg.Ver = m.Version

	return pkg
}

func finalizePkg(pkg *ParsedPackage) (*ParsedPackage, error) {
	deps, err := determineDeps(pkg.MetaObj)
	if err != nil {
		return nil, err
	}

	pkg.Deps = deps

	return pkg, nil
}

func determineDeps(o runtime.Object) ([]v1beta1.Dependency, error) {
	pkg, ok := scheme.TryConvertToPkg(o, &xpmetav1.Provider{}, &xpmetav1.Configuration{}, &xpmetav1.Function{})
	if !ok {
		return nil, errors.New(errFailedToConvertMetaToPackage)
	}

	out := make([]v1beta1.Dependency, len(pkg.GetDependencies()))
	for i, d := range pkg.GetDependencies() {
		out[i] = convertToV1beta1(d)
	}

	return out, nil
}

func convertToV1beta1(in xpmetav1.Dependency) v1beta1.Dependency {
	betaD := v1beta1.Dependency{
		Constraints: in.Version,
	}

	switch {
	case in.APIVersion != nil && in.Kind != nil && in.Package != nil:
		betaD.APIVersion = in.APIVersion
		betaD.Kind = in.Kind
		betaD.Package = *in.Package

	case in.Provider != nil:
		betaD.Package = *in.Provider
		betaD.Type = ptr.To(v1beta1.ProviderPackageType)

	case in.Configuration != nil:
		betaD.Package = *in.Configuration
		betaD.Type = ptr.To(v1beta1.ConfigurationPackageType)

	case in.Function != nil:
		betaD.Package = *in.Function
		betaD.Type = ptr.To(v1beta1.FunctionPackageType)
	}

	return betaD
}
