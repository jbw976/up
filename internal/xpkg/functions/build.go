// Copyright 2025 Upbound Inc.
// All rights reserved

// Package functions contains functions for building functions
package functions

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"slices"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/ko/pkg/build"
	"github.com/spf13/afero"
	"golang.org/x/sync/errgroup"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/internal/xpkg"
)

const (
	errNoSuitableBuilder        = "no suitable builder found"
	crossplaneFunctionRunnerUID = 2000
	crossplaneFunctionRunnerGID = 2000
)

// Identifier knows how to identify an appropriate builder for a function based
// on it source code.
type Identifier interface {
	// Identify returns a suitable builder for the function whose source lives
	// in the given filesystem. It returns an error if no such builder is
	// available.
	Identify(fromFS afero.Fs) (Builder, error)
}

type realIdentifier struct{}

// DefaultIdentifier is the default builder identifier, suitable for production
// use.
//
//nolint:gochecknoglobals // we want to keep this global
var DefaultIdentifier = realIdentifier{}

func (realIdentifier) Identify(fromFS afero.Fs) (Builder, error) {
	// builders are the known builder types, in order of precedence.
	builders := []Builder{
		newKCLBuilder(),
		newPythonBuilder(),
		newGoBuilder(),
	}
	for _, b := range builders {
		ok, err := b.match(fromFS)
		if err != nil {
			return nil, errors.Wrapf(err, "builder %q returned an error", b.Name())
		}
		if ok {
			return b, nil
		}
	}

	return nil, errors.New(errNoSuitableBuilder)
}

type nopIdentifier struct{}

// FakeIdentifier is an identifier that always returns a fake builder. This is
// for use in tests where we don't want to do real builds.
//
//nolint:gochecknoglobals // we want to keep this global
var FakeIdentifier = nopIdentifier{}

func (nopIdentifier) Identify(_ afero.Fs) (Builder, error) {
	return &fakeBuilder{}, nil
}

// Builder knows how to build a particular kind of function.
type Builder interface {
	// Name returns a name for this builder.
	Name() string
	// Build builds the function whose source lives in the given filesystem,
	// returning an image for each architecture. This image will *not* include
	// package metadata; it's just the runtime image for the function.
	Build(ctx context.Context, fromFS afero.Fs, architectures []string, osBasePath string) ([]v1.Image, error)

	// match returns true if this builder can build the function whose source
	// lives in the given filesystem.
	match(fromFS afero.Fs) (bool, error)
}

// kclBuilder builds functions written in KCL by injecting their code into a
// function-kcl base image.
type kclBuilder struct {
	baseImage string
	transport http.RoundTripper
}

func (b *kclBuilder) Name() string {
	return "kcl"
}

func (b *kclBuilder) match(fromFS afero.Fs) (bool, error) {
	return afero.Exists(fromFS, "kcl.mod")
}

func (b *kclBuilder) Build(ctx context.Context, fromFS afero.Fs, architectures []string, osBasePath string) ([]v1.Image, error) {
	baseRef, err := name.NewTag(b.baseImage)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse KCL base image tag")
	}

	images := make([]v1.Image, len(architectures))
	eg, _ := errgroup.WithContext(ctx)
	for i, arch := range architectures {
		eg.Go(func() error {
			baseImg, err := baseImageForArch(baseRef, arch, b.transport)
			if err != nil {
				return errors.Wrap(err, "failed to fetch KCL base image")
			}

			src, err := filesystem.FSToTar(fromFS, "/src",
				filesystem.WithSymlinkBasePath(osBasePath),
				// The KCL base function implementation requires that the source
				// files (specifically, the kcl.mod.lock) be writable. Make them
				// owned by the UID/GID that crossplane uses to run function
				// pods.
				filesystem.WithUIDOverride(crossplaneFunctionRunnerUID),
				filesystem.WithGIDOverride(crossplaneFunctionRunnerGID),
			)
			if err != nil {
				return errors.Wrap(err, "failed to tar layer contents")
			}

			codeLayer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(src)), nil
			})
			if err != nil {
				return errors.Wrap(err, "failed to create code layer")
			}

			img, err := mutate.AppendLayers(baseImg, codeLayer)
			if err != nil {
				return errors.Wrap(err, "failed to add code to image")
			}

			// Set the default source to match our source directory.
			img, err = setImageEnvvar(img, "FUNCTION_KCL_DEFAULT_SOURCE", "/src")
			if err != nil {
				return errors.Wrap(err, "failed to configure KCL source path")
			}

			images[i] = img
			return nil
		})
	}

	return images, eg.Wait()
}

// pythonBuilder builds functions written in python by injecting their code into a
// function-python base image.
type pythonBuilder struct {
	baseImage   string
	packagePath string
	transport   http.RoundTripper
}

func (b *pythonBuilder) Name() string {
	return "python"
}

func (b *pythonBuilder) match(fromFS afero.Fs) (bool, error) {
	// More reliable than requirements.txt, which is optional.
	return afero.Exists(fromFS, "main.py")
}

func (b *pythonBuilder) Build(ctx context.Context, fromFS afero.Fs, architectures []string, osBasePath string) ([]v1.Image, error) {
	baseRef, err := name.NewTag(b.baseImage)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse python base image tag")
	}

	images := make([]v1.Image, len(architectures))
	eg, _ := errgroup.WithContext(ctx)
	for i, arch := range architectures {
		eg.Go(func() error {
			baseImg, err := remote.Image(baseRef, remote.WithPlatform(v1.Platform{
				OS:           "linux",
				Architecture: arch,
			}), remote.WithTransport(b.transport))
			if err != nil {
				return errors.Wrap(err, "failed to fetch python base image")
			}

			src, err := filesystem.FSToTar(fromFS, b.packagePath,
				filesystem.WithSymlinkBasePath(osBasePath),
			)
			if err != nil {
				return errors.Wrap(err, "failed to tar layer contents")
			}

			codeLayer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(src)), nil
			})
			if err != nil {
				return errors.Wrap(err, "failed to create code layer")
			}

			img, err := mutate.AppendLayers(baseImg, codeLayer)
			if err != nil {
				return errors.Wrap(err, "failed to add code to image")
			}

			images[i] = img
			return nil
		})
	}

	return images, eg.Wait()
}

// goBuilder builds functions written in Go using ko.
type goBuilder struct {
	baseImage string
	transport http.RoundTripper
}

func (b *goBuilder) Name() string {
	return "go"
}

func (b *goBuilder) match(fromFS afero.Fs) (bool, error) {
	return afero.Exists(fromFS, "go.mod")
}

func (b *goBuilder) Build(ctx context.Context, _ afero.Fs, architectures []string, osBasePath string) ([]v1.Image, error) {
	// ko logs using the Go standard library global logger, and doesn't provide
	// any option to disable output. Disable output while we do our builds so we
	// don't show the user a bunch of ko junk. We don't use the standard logger
	// at all in `up`, so it's fine to leave it disabled.
	log.SetOutput(io.Discard)

	platforms := make([]string, len(architectures))
	for i, arch := range architectures {
		platforms[i] = "linux/" + arch
	}

	builder, err := build.NewGo(ctx, osBasePath,
		build.WithBaseImages(func(_ context.Context, _ string) (name.Reference, build.Result, error) {
			ref, err := name.ParseReference(b.baseImage)
			if err != nil {
				return nil, nil, err
			}
			img, err := remote.Index(ref, remote.WithTransport(b.transport))
			return ref, img, err
		}),
		build.WithPlatforms(platforms...),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to construct ko builder")
	}
	builder, err = build.NewCaching(builder)
	if err != nil {
		return nil, errors.Wrap(err, "failed to construct caching builder")
	}

	path, err := builder.QualifyImport(".")
	if err != nil {
		return nil, errors.Wrap(err, "failed to determine go module path for function")
	}

	res, err := builder.Build(ctx, path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build function")
	}

	// ko will return an index if we're building multiple platforms and an image
	// if we're building only one platform, so we need to handle both return
	// types.
	var imgs []v1.Image
	switch out := res.(type) {
	case v1.ImageIndex:
		idx, err := out.IndexManifest()
		if err != nil {
			return nil, errors.Wrap(err, "failed to get index manifest")
		}

		imgs = make([]v1.Image, len(idx.Manifests))
		for i, desc := range idx.Manifests {
			img, err := out.Image(desc.Digest)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to get image %v from index", desc.Digest)
			}
			imgs[i] = img
		}

	case v1.Image:
		imgs = []v1.Image{out}

	default:
		return nil, errors.Errorf("ko builder returned unexpected type %T", res)
	}

	return imgs, nil
}

func newGoBuilder() *goBuilder {
	return &goBuilder{
		baseImage: "xpkg.upbound.io/upbound/provider-base@sha256:d23697e028f65fcc35886fe9e875069c071f637a79d65821830d6bc71c975391",
		transport: http.DefaultTransport,
	}
}

// baseImageForArch pulls the image with the given ref, and returns a version of
// it suitable for use as a function base image. Specifically, the package
// layer, examples layer, and schema layers will be removed if present. Note
// that layers in the returned image will refer to the remote and be pulled only
// if they are read by the caller.
func baseImageForArch(ref name.Reference, arch string, transport http.RoundTripper) (v1.Image, error) {
	img, err := remote.Image(ref, remote.WithPlatform(v1.Platform{
		OS:           "linux",
		Architecture: arch,
	}), remote.WithTransport(transport))
	if err != nil {
		return nil, errors.Wrap(err, "failed to pull image")
	}

	cfg, err := img.ConfigFile()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get config from image")
	}
	if cfg.Architecture != arch {
		return nil, errors.Errorf("image not available for architecture %q", arch)
	}

	// Remove the package layer and schema layers if present.
	mfst, err := img.Manifest()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get manifest from image")
	}
	baseImage := empty.Image
	// The RootFS contains a list of layers; since we're removing layers we need
	// to clear it out. It will be rebuilt by the mutate package.
	cfg.RootFS = v1.RootFS{}
	cfg.History = nil
	baseImage, err = mutate.ConfigFile(baseImage, cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to add configuration to base image")
	}
	for _, desc := range mfst.Layers {
		if isNonBaseLayer(desc) {
			continue
		}
		l, err := img.LayerByDigest(desc.Digest)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get layer from image")
		}
		baseImage, err = mutate.AppendLayers(baseImage, l)
		if err != nil {
			return nil, errors.Wrap(err, "failed to add layer to base image")
		}
	}

	return baseImage, nil
}

func isNonBaseLayer(desc v1.Descriptor) bool {
	nonBaseLayerAnns := []string{
		xpkg.PackageAnnotation,
		xpkg.ExamplesAnnotation,
		xpkg.SchemaKclAnnotation,
		xpkg.SchemaPythonAnnotation,
	}

	ann := desc.Annotations[xpkg.AnnotationKey]
	return slices.Contains(nonBaseLayerAnns, ann)
}

func setImageEnvvar(image v1.Image, key string, value string) (v1.Image, error) {
	cfgFile, err := image.ConfigFile()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get config file")
	}
	cfg := cfgFile.Config
	cfg.Env = append(cfg.Env, fmt.Sprintf("%s=%s", key, value))

	image, err = mutate.Config(image, cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to set config")
	}

	return image, nil
}

func newKCLBuilder() *kclBuilder {
	return &kclBuilder{
		baseImage: "xpkg.upbound.io/upbound/function-kcl-base:v0.10.8-up.2",
		transport: http.DefaultTransport,
	}
}

func newPythonBuilder() *pythonBuilder {
	return &pythonBuilder{
		// TODO(negz): Should this be hardcoded?
		baseImage: "xpkg.upbound.io/upbound/function-interpreter-python:v0.3.0",

		// TODO(negz): This'll need to change if function-interpreter-python is
		// updated to a distroless base layer that uses a newer Python version.
		packagePath: "/venv/fn/lib/python3.11/site-packages/function",
		transport:   http.DefaultTransport,
	}
}

// fakeBuilder builds empty images with correct configs. It is intended for use
// in unit tests. It matches any input.
type fakeBuilder struct{}

func (b *fakeBuilder) Name() string {
	return "fake"
}

func (b *fakeBuilder) match(_ afero.Fs) (bool, error) {
	return true, nil
}

func (b *fakeBuilder) Build(_ context.Context, _ afero.Fs, architectures []string, _ string) ([]v1.Image, error) {
	images := make([]v1.Image, len(architectures))
	for i, arch := range architectures {
		baseImg := empty.Image
		cfg := &v1.ConfigFile{
			OS:           "linux",
			Architecture: arch,
		}
		img, err := mutate.ConfigFile(baseImg, cfg)
		if err != nil {
			return nil, err
		}
		images[i] = img
	}

	return images, nil
}
