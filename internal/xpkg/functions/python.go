// Copyright 2025 Upbound Inc.
// All rights reserved

package functions

import (
	"bytes"
	"context"
	"io"
	"net/http"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/spf13/afero"
	"golang.org/x/sync/errgroup"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/internal/imageutil"
	"github.com/upbound/up/internal/upbound"
	projectv1alpha1 "github.com/upbound/up/pkg/apis/project/v1alpha1"
)

// pythonBuilder builds functions written in python by injecting their code into a
// function-python base image.
type pythonBuilder struct {
	baseImage    string
	packagePath  string
	transport    http.RoundTripper
	imageConfigs []projectv1alpha1.ImageConfig
	upCtx        *upbound.Context
}

func (b *pythonBuilder) Name() string {
	return "python"
}

func (b *pythonBuilder) match(fromFS afero.Fs) (bool, error) {
	// More reliable than requirements.txt, which is optional.
	return afero.Exists(fromFS, "main.py")
}

func (b *pythonBuilder) Build(ctx context.Context, fromFS afero.Fs, architectures []string, osBasePath string) ([]v1.Image, error) {
	baseImage := b.baseImage
	if len(b.imageConfigs) > 0 {
		baseImage = imageutil.RewriteImage(b.baseImage, b.imageConfigs)
	}

	baseRef, err := name.NewTag(baseImage)
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
			}), remote.WithTransport(b.transport), remote.WithAuthFromKeychain(b.upCtx.RegistryKeychain()))
			if err != nil {
				return errors.Wrap(err, "failed to fetch python base image")
			}

			src, err := filesystem.FSToTar(fromFS, b.packagePath,
				filesystem.WithSymlinkBasePath(osBasePath),
				// Files might not be world-readable in the local filesystem, so
				// make them owned by the user that needs to read them in the
				// function pod to ensure they can be used by the interpreter.
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

			images[i] = img
			return nil
		})
	}

	return images, eg.Wait()
}

func newPythonBuilder(imageConfigs []projectv1alpha1.ImageConfig, upCtx *upbound.Context) *pythonBuilder {
	return &pythonBuilder{
		// TODO(negz): Should this be hardcoded?
		baseImage: "xpkg.upbound.io/upbound/function-interpreter-python:v0.4.0",

		// TODO(negz): This'll need to change if function-interpreter-python is
		// updated to a distroless base layer that uses a newer Python version.
		packagePath:  "/venv/fn/lib/python3.11/site-packages/function",
		transport:    http.DefaultTransport,
		imageConfigs: imageConfigs,
		upCtx:        upCtx,
	}
}
