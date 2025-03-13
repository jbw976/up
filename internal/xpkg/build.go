// Copyright 2025 Upbound Inc.
// All rights reserved

package xpkg

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"gopkg.in/yaml.v2"
	crd "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/parser"
	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"

	upboundpkgmetav1alpha1 "github.com/upbound/up-sdk-go/apis/pkg/meta/v1alpha1"
	"github.com/upbound/up/internal/xpkg/parser/examples"
	"github.com/upbound/up/internal/xpkg/parser/linter"
	"github.com/upbound/up/internal/xpkg/scheme"
)

const (
	errParserPackage     = "failed to parse package"
	errParserExample     = "failed to parse examples"
	errLintPackage       = "failed to lint package"
	errInitBackend       = "failed to initialize package parsing backend"
	errInitHelmBackend   = "failed to initialize helm backend"
	errBuildImage        = "failed to build image"
	errDigestInvalid     = "digest is invalid"
	errLayerFromTar      = "failed to create layer from tar"
	errTarFromStream     = "failed to create tar from stream"
	errConfigFile        = "failed to get config file"
	errMutateConfig      = "failed to mutate config"
	errParseAuth         = "failed to parse auth"
	errAuthNotAnnotated  = "failed to annotate auth"
	errControllerNoHelm  = "controller package requires a helm chart"
	errBuildObjectScheme = "failed to build scheme for package encoder"
	authMetaAnno         = "auth.upbound.io/group"
	// AuthObjectAnno is the auth object annotation.
	AuthObjectAnno = "auth.upbound.io/config"
	// ProviderConfigKind is the kind of the provider config.
	ProviderConfigKind = "ProviderConfig"
)

// Mutator defines a mutation / add additional layers on an image and its config.
type Mutator interface {
	Mutate(i v1.Image, c v1.Config) (v1.Image, v1.Config, error)
}

// annotatedTeeReadCloser is a copy of io.TeeReader that implements
// parser.AnnotatedReadCloser. It returns a Reader that writes to w what it
// reads from r. All reads from r performed through it are matched with
// corresponding writes to w. There is no internal buffering - the write must
// complete before the read completes. Any error encountered while writing is
// reported as a read error. If the underlying reader is a
// parser.AnnotatedReadCloser the tee reader will invoke its Annotate function.
// Otherwise it will return nil. Closing is always a no-op.
func annotatedTeeReadCloser(r io.Reader, w io.Writer) *teeReader {
	return &teeReader{r, w}
}

type teeReader struct {
	r io.Reader
	w io.Writer
}

func (t *teeReader) Read(p []byte) (n int, err error) {
	n, err = t.r.Read(p)
	if n > 0 {
		if n, err := t.w.Write(p[:n]); err != nil {
			return n, err
		}
	}
	return
}

func (t *teeReader) Close() error {
	return nil
}

func (t *teeReader) Annotate() any {
	anno, ok := t.r.(parser.AnnotatedReadCloser)
	if !ok {
		return nil
	}
	return anno.Annotate()
}

// Builder defines an xpkg Builder.
type Builder struct {
	packageBackend  parser.Backend
	examplesBackend parser.Backend
	authBackend     parser.Backend
	helmBackend     parser.Backend
	packageParser   parser.Parser
	examplesParser  *examples.Parser
	mutators        []Mutator
}

// New returns a new Builder.
func New(pkg, ab, ex, h parser.Backend, pp parser.Parser, ep *examples.Parser, mutators ...Mutator) *Builder {
	return &Builder{
		packageBackend:  pkg,
		authBackend:     ab,
		examplesBackend: ex,
		helmBackend:     h,
		packageParser:   pp,
		examplesParser:  ep,
		mutators:        mutators,
	}
}

type buildOpts struct {
	base v1.Image
}

// A BuildOpt modifies how a package is built.
type BuildOpt func(*buildOpts)

// WithController sets the controller image that should serve as the base for
// the package.
func WithController(img v1.Image) BuildOpt {
	return func(o *buildOpts) {
		o.base = img
	}
}

// AuthExtension is the structure of the auth.yaml file.
type AuthExtension struct {
	Version      string `yaml:"version"`
	Discriminant string `yaml:"discriminant"`
	Sources      []struct {
		Name                string `yaml:"name"`
		Docs                string `yaml:"docs"`
		AdditionalResources []struct {
			Type string `yaml:"type"`
			Ref  string `yaml:"ref"`
		} `yaml:"additionalResources"`
		ShowFields []string `yaml:"showFields"`
	} `yaml:"sources"`
}

// Build compiles a Crossplane package from an on-disk package.
func (b *Builder) Build(ctx context.Context, opts ...BuildOpt) (v1.Image, runtime.Object, error) { //nolint:gocognit // TODO: refactor
	bOpts := &buildOpts{
		base: empty.Image,
	}
	for _, o := range opts {
		o(bOpts)
	}

	// assume examples exist
	examplesExist := true
	// Get package YAML stream.
	pkgReader, err := b.packageBackend.Init(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, errInitBackend)
	}
	defer func() { _ = pkgReader.Close() }()

	// Get examples YAML stream.
	exReader, err := b.examplesBackend.Init(ctx)
	if err != nil && !os.IsNotExist(err) {
		return nil, nil, errors.Wrap(err, errInitBackend)
	}
	defer func() { _ = exReader.Close() }()
	// examples/ doesn't exist
	if os.IsNotExist(err) {
		examplesExist = false
	}

	var helmReader io.ReadCloser
	if b.helmBackend != nil {
		helmReader, err = b.helmBackend.Init(ctx)
		if err != nil && !os.IsNotExist(err) {
			return nil, nil, errors.Wrap(err, errInitHelmBackend)
		}
		defer func() { _ = helmReader.Close() }()
	}

	pkg, err := b.packageParser.Parse(ctx, pkgReader)
	if err != nil {
		return nil, nil, errors.Wrap(err, errParserPackage)
	}

	metas := pkg.GetMeta()
	if len(metas) != 1 {
		return nil, nil, errors.New(errNotExactlyOneMeta)
	}

	// TODO(hasheddan): make linter selection logic configurable.
	meta := metas[0]
	var linter linter.Linter
	switch meta.GetObjectKind().GroupVersionKind().Kind {
	case pkgmetav1.ConfigurationKind:
		linter = NewConfigurationLinter()
	case v1beta1.FunctionKind:
		linter = NewFunctionLinter()
	case pkgmetav1.ProviderKind:
		if b.authBackend != nil { // if we have an auth.yaml file
			if p, ok := meta.(metav1.Object); ok {
				if group, ok := p.GetAnnotations()[authMetaAnno]; ok {
					// if we found an annotation auth.upbound.io/group then look for the object
					// specified there like aws.upbound.io and annotate that with auth.upbound.io/config
					// and embed the contents of the auth.yaml file
					ar, err := b.authBackend.Init(ctx)
					if err != nil {
						return nil, nil, errors.Wrap(err, errParseAuth)
					}
					// validate the auth.yaml file
					var auth AuthExtension
					if err := yaml.NewDecoder(ar).Decode(&auth); err != nil {
						return nil, nil, errors.Wrap(err, errParseAuth)
					}
					annotated := false
					for x, o := range pkg.GetObjects() {
						if c, ok := o.(*crd.CustomResourceDefinition); ok {
							if c.Spec.Group == group && c.Spec.Names.Kind == ProviderConfigKind {
								ab := new(bytes.Buffer)
								if err := yaml.NewEncoder(ab).Encode(auth); err != nil {
									return nil, nil, errors.Wrap(err, errParseAuth)
								}
								c.Annotations[AuthObjectAnno] = ab.String()
								pkg.GetObjects()[x] = c
								annotated = true
								break
							}
						}
					}
					if !annotated {
						return nil, nil, errors.New(errAuthNotAnnotated)
					}
				}
			}
		}
		linter = NewProviderLinter()
	case upboundpkgmetav1alpha1.ControllerKind:
		linter = NewControllerLinter()
	}
	if err := linter.Lint(pkg); err != nil {
		return nil, nil, errors.Wrap(err, errLintPackage)
	}

	layers := make([]v1.Layer, 0)
	cfgFile, err := bOpts.base.ConfigFile()
	if err != nil {
		return nil, nil, errors.Wrap(err, errConfigFile)
	}

	cfg := cfgFile.Config
	cfg.Labels = make(map[string]string)

	pkgBytes, err := encode(pkg)
	if err != nil {
		return nil, nil, errors.Wrap(err, errConfigFile)
	}

	pkgLayer, err := Layer(pkgBytes, StreamFile, PackageAnnotation, int64(pkgBytes.Len()), StreamFileMode, &cfg)
	if err != nil {
		return nil, nil, err
	}
	layers = append(layers, pkgLayer)

	// examples exist, create the layer
	if examplesExist {
		exBuf := new(bytes.Buffer)
		if _, err = b.examplesParser.Parse(ctx, annotatedTeeReadCloser(exReader, exBuf)); err != nil {
			return nil, nil, errors.Wrap(err, errParserExample)
		}

		exLayer, err := Layer(exBuf, XpkgExamplesFile, ExamplesAnnotation, int64(exBuf.Len()), StreamFileMode, &cfg)
		if err != nil {
			return nil, nil, err
		}
		layers = append(layers, exLayer)
	}

	if meta.GetObjectKind().GroupVersionKind().Kind == upboundpkgmetav1alpha1.ControllerKind {
		// Controller packages must have a helm chart
		if helmReader == nil {
			return nil, nil, errors.New(errControllerNoHelm)
		}
		// Create the helm layer from the helm chart
		helmBuf := new(bytes.Buffer)
		if _, err := io.Copy(helmBuf, helmReader); err != nil {
			return nil, nil, errors.Wrap(err, "failed to read helm chart")
		}
		helmLayer, err := Layer(helmBuf, XpkgHelmChartFile, HelmChartAnnotation, int64(helmBuf.Len()), StreamFileMode, &cfg)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to create helm layer")
		}
		layers = append(layers, helmLayer)
	}

	for _, mut := range b.mutators {
		if mut != nil {
			bOpts.base, cfg, err = mut.Mutate(bOpts.base, cfg)
			if err != nil {
				return nil, nil, errors.Wrap(err, "failed to apply mutator")
			}
		}
	}

	for _, l := range layers {
		bOpts.base, err = mutate.AppendLayers(bOpts.base, l)
		if err != nil {
			return nil, nil, errors.Wrap(err, errBuildImage)
		}
	}

	bOpts.base, err = mutate.Config(bOpts.base, cfg)
	if err != nil {
		return nil, nil, errors.Wrap(err, errMutateConfig)
	}

	return bOpts.base, meta, nil
}

// encode encodes a package as a YAML stream.  Does not check meta existence
// or quantity i.e. it should be linted first to ensure that it is valid.
func encode(pkg linter.Package) (*bytes.Buffer, error) {
	pkgBuf := new(bytes.Buffer)
	objScheme, err := scheme.BuildObjectScheme()
	if err != nil {
		return nil, errors.New(errBuildObjectScheme)
	}

	do := json.NewSerializerWithOptions(json.DefaultMetaFactory, objScheme, objScheme, json.SerializerOptions{Yaml: true})
	pkgBuf.WriteString("---\n")
	if err = do.Encode(pkg.GetMeta()[0], pkgBuf); err != nil {
		return nil, errors.Wrap(err, errBuildObjectScheme)
	}
	pkgBuf.WriteString("---\n")
	for _, o := range pkg.GetObjects() {
		if err = do.Encode(o, pkgBuf); err != nil {
			return nil, errors.Wrap(err, errBuildObjectScheme)
		}
		pkgBuf.WriteString("---\n")
	}
	return pkgBuf, nil
}

// SkipContains supplies a FilterFn that skips paths that contain the give pattern.
func SkipContains(pattern string) parser.FilterFn {
	return func(path string, _ os.FileInfo) (bool, error) {
		return strings.Contains(path, pattern), nil
	}
}
