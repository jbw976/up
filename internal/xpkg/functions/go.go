// Copyright 2025 Upbound Inc.
// All rights reserved

package functions

import (
	"context"
	"io"
	"log"
	"net/http"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/ko/pkg/build"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/imageutil"
	projectv1alpha1 "github.com/upbound/up/pkg/apis/project/v1alpha1"
)

// goBuilder builds functions written in Go using ko.
type goBuilder struct {
	baseImage    string
	transport    http.RoundTripper
	imageConfigs []projectv1alpha1.ImageConfig
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
			baseImage := b.baseImage
			if len(b.baseImage) > 0 {
				baseImage = imageutil.RewriteImage(b.baseImage, b.imageConfigs)
			}
			ref, err := name.ParseReference(baseImage, name.StrictValidation)
			if err != nil {
				return nil, nil, err
			}
			img, err := remote.Index(ref, remote.WithTransport(b.transport), remote.WithAuthFromKeychain(authn.NewMultiKeychain(authn.DefaultKeychain)))
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

func newGoBuilder(imageConfigs []projectv1alpha1.ImageConfig) *goBuilder {
	return &goBuilder{
		baseImage:    "xpkg.upbound.io/upbound/provider-base@sha256:d23697e028f65fcc35886fe9e875069c071f637a79d65821830d6bc71c975391",
		transport:    http.DefaultTransport,
		imageConfigs: imageConfigs,
	}
}
