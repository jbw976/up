// Copyright 2025 Upbound Inc.
// All rights reserved

package functions

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"slices"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/spf13/afero"
	"golang.org/x/sync/errgroup"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/internal/imageutil"
	"github.com/upbound/up/internal/upbound"
	projectv1alpha1 "github.com/upbound/up/pkg/apis/project/v1alpha1"
)

// goTemplatingBuilder builds "functions" written in go templating by injecting
// their code into a function-go-templating base image.
type goTemplatingBuilder struct {
	baseImage    string
	transport    http.RoundTripper
	imageConfigs []projectv1alpha1.ImageConfig
	upCtx        *upbound.Context
}

func (b *goTemplatingBuilder) Name() string {
	return "go-templating"
}

func (b *goTemplatingBuilder) match(fromFS afero.Fs) (bool, error) {
	goTemplatingExtensions := []string{
		".gotmpl",
		".tmpl",
	}

	// The go templating builder will match any directory containing only files
	// with recognized extensions. Nested directories are allowed. An empty
	// directory is not matched.
	matches := false
	err := afero.Walk(fromFS, ".", func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Ignore directories but recurse into them.
		if info.Mode().IsDir() {
			return nil
		}

		// Don't support symlinks or any other funky stuff. We don't need to
		// symlink in models like we do for other languages, so it's simplest to
		// just not support them.
		if !info.Mode().IsRegular() {
			matches = false
			return fs.SkipAll
		}

		if !slices.Contains(goTemplatingExtensions, filepath.Ext(path)) {
			matches = false
			return fs.SkipAll
		}

		matches = true
		return nil
	})

	if errors.Is(err, fs.SkipAll) {
		err = nil
	}

	return matches, err
}

func (b *goTemplatingBuilder) Build(ctx context.Context, fromFS afero.Fs, architectures []string, osBasePath string) ([]v1.Image, error) {
	baseImage := b.baseImage
	if len(b.imageConfigs) > 0 {
		baseImage = imageutil.RewriteImage(b.baseImage, b.imageConfigs)
	}
	baseRef, err := name.NewTag(baseImage)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse go-templating base image tag")
	}

	images := make([]v1.Image, len(architectures))
	eg, _ := errgroup.WithContext(ctx)
	for i, arch := range architectures {
		eg.Go(func() error {
			baseImg, err := baseImageForArch(baseRef, arch, b.transport, b.upCtx)
			if err != nil {
				return errors.Wrap(err, "failed to fetch go-templating base image")
			}

			src, err := filesystem.FSToTar(fromFS, "/src",
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

			// Set the default source to match our source directory.
			img, err = setImageEnvvars(img, map[string]string{
				"FUNCTION_GO_TEMPLATING_DEFAULT_SOURCE": "/src",
			})
			if err != nil {
				return errors.Wrap(err, "failed to configure go-templating source path")
			}

			images[i] = img
			return nil
		})
	}

	return images, eg.Wait()
}

func newGoTemplatingBuilder(imageConfigs []projectv1alpha1.ImageConfig, upCtx *upbound.Context) *goTemplatingBuilder {
	return &goTemplatingBuilder{
		// TODO(adamwg): Upstream changes and switch to the official function.
		transport:    http.DefaultTransport,
		baseImage:    "xpkg.upbound.io/upbound/function-go-templating-base:v0.9.0-13-gd1fa2e3",
		imageConfigs: imageConfigs,
		upCtx:        upCtx,
	}
}
