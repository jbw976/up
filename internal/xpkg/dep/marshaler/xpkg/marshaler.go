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
	schemas := make(map[string]afero.Fs)

	for _, l := range manifest.Layers {
		if val, ok := l.Annotations[xpkg.AnnotationKey]; ok && val == xpkg.PackageAnnotation {
			packageLayerDigest = l.Digest
		}

		// Dynamically detect schema annotations (e.g., schema.python, schema.kcl, etc.)
		for _, annotationValue := range l.Annotations {
			if strings.HasPrefix(annotationValue, "schema.") {
				lang := strings.TrimPrefix(annotationValue, "schema.")
				// Copy the files into an in-memory FS. Ideally we'd use the
				// tarfs here, but afero.Walk doesn't work on tarfs if the
				// underlying tarball doesn't have explicit directory entries,
				// and we can't guarantee that our tarballs have those.
				mfs := afero.NewMemMapFs()
				if err := extractLayerToFs(i, l.Digest, mfs); err != nil {
					return nil, err
				}
				schemas[lang] = mfs
			}
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
	pkg.Schema = schemas

	return finalizePkg(pkg)
}

func extractLayerToFs(i xpkg.Image, layerDigest cv1.Hash, fs afero.Fs) error {
	targetLayer, err := i.Image.LayerByDigest(layerDigest)
	if err != nil {
		return errors.Wrapf(err, "failed to get layer %s", layerDigest)
	}

	reader, err := targetLayer.Uncompressed()
	if err != nil {
		return errors.Wrap(err, "failed to extract target layer")
	}

	tarReader := tar.NewReader(reader)

	// Iterate over the files in the tarball and write them to the Afero filesystem
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return errors.Wrap(err, "failed to read tar header")
		}

		// Construct the full output path for the file in the Afero filesystem
		outputPath := header.Name

		switch header.Typeflag {
		case tar.TypeDir:
			// Create directories in the Afero filesystem
			if err := fs.MkdirAll(outputPath, 0o755); err != nil {
				return errors.Wrapf(err, "failed to create directory %s", outputPath)
			}
		case tar.TypeReg:
			// Limit the number of bytes copied from the tarReader to prevent decompression bombs
			lr := io.LimitReader(tarReader, maxFileSize)
			if err := afero.WriteReader(fs, outputPath, lr); err != nil {
				return errors.Wrapf(err, "failed to write file %s", outputPath)
			}
		}
	}

	return nil
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

	schemas, err := r.loadSchemasFromDir(fs, path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load schema directories")
	}
	pkg.Schema = schemas

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
	var pkgType v1beta1.PackageType
	switch meta.GetObjectKind().GroupVersionKind().Kind {
	case xpmetav1.ConfigurationKind:
		linter = xpkg.NewConfigurationLinter()
		pkgType = v1beta1.ConfigurationPackageType
	case xpmetav1.ProviderKind:
		linter = xpkg.NewProviderLinter()
		pkgType = v1beta1.ProviderPackageType
	case xpmetav1beta1.FunctionKind:
		linter = xpkg.NewFunctionLinter()
		pkgType = v1beta1.FunctionPackageType
	}
	if err := linter.Lint(pkg); err != nil {
		return nil, errors.Wrap(err, errLintPackage)
	}

	return &ParsedPackage{
		MetaObj: meta,
		Objs:    pkg.GetObjects(),
		PType:   pkgType,
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

func (r *Marshaler) loadSchemasFromDir(fs afero.Fs, path string) (map[string]afero.Fs, error) {
	schemas := make(map[string]afero.Fs)

	// Read the contents of the directory
	dirEntries, err := afero.ReadDir(fs, path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read directory")
	}

	// Iterate through the directory contents to find schema.* folders
	for _, entry := range dirEntries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "schema.") {
			lang := strings.TrimPrefix(entry.Name(), "schema.")
			schemas[lang] = afero.NewBasePathFs(fs, filepath.Join(path, entry.Name()))
		}
	}

	return schemas, nil
}
