// Copyright 2025 Upbound Inc.
// All rights reserved

package functions

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"slices"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/spf13/afero"
	"golang.org/x/sync/errgroup"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/internal/imageutil"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/xpkg"
	projectv1alpha1 "github.com/upbound/up/pkg/apis/project/v1alpha1"
)

const (
	crossplaneFunctionRunnerUID = 2000
	crossplaneFunctionRunnerGID = 2000
)

// kclBuilder builds functions written in KCL by injecting their code into a
// function-kcl base image.
type kclBuilder struct {
	baseImage    string
	transport    http.RoundTripper
	imageConfigs []projectv1alpha1.ImageConfig
	upCtx        *upbound.Context
}

func (b *kclBuilder) Name() string {
	return "kcl"
}

func (b *kclBuilder) match(fromFS afero.Fs) (bool, error) {
	return afero.Exists(fromFS, "kcl.mod")
}

func (b *kclBuilder) Build(ctx context.Context, fromFS afero.Fs, architectures []string, osBasePath string) ([]v1.Image, error) {
	baseImage := b.baseImage
	if len(b.imageConfigs) > 0 {
		baseImage = imageutil.RewriteImage(b.baseImage, b.imageConfigs)
	}
	baseRef, err := name.NewTag(baseImage)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse KCL base image tag")
	}

	images := make([]v1.Image, len(architectures))
	eg, _ := errgroup.WithContext(ctx)
	for i, arch := range architectures {
		eg.Go(func() error {
			baseImg, err := baseImageForArch(baseRef, arch, b.transport, b.upCtx)
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
			img, err = setImageEnvvars(img, map[string]string{
				"FUNCTION_KCL_DEFAULT_SOURCE": "/src",
				"KCL_PKG_PATH":                "/src",
			})
			if err != nil {
				return errors.Wrap(err, "failed to configure KCL source path")
			}

			images[i] = img
			return nil
		})
	}

	return images, eg.Wait()
}

// baseImageForArch pulls the image with the given ref, and returns a version of
// it suitable for use as a function base image. Specifically, the package
// layer, examples layer, and schema layers will be removed if present. Note
// that layers in the returned image will refer to the remote and be pulled only
// if they are read by the caller.
func baseImageForArch(ref name.Reference, arch string, transport http.RoundTripper, upCtx *upbound.Context) (v1.Image, error) {
	img, err := remote.Image(ref, remote.WithPlatform(v1.Platform{
		OS:           "linux",
		Architecture: arch,
	}), remote.WithTransport(transport), remote.WithAuthFromKeychain(upCtx.RegistryKeychain()))
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

func setImageEnvvars(image v1.Image, envVars map[string]string) (v1.Image, error) {
	cfgFile, err := image.ConfigFile()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get config file")
	}
	cfg := cfgFile.Config

	for k, v := range envVars {
		cfg.Env = append(cfg.Env, fmt.Sprintf("%s=%s", k, v))
	}

	image, err = mutate.Config(image, cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to set config")
	}

	return image, nil
}

func newKCLBuilder(imageConfigs []projectv1alpha1.ImageConfig, upCtx *upbound.Context) *kclBuilder {
	return &kclBuilder{
		baseImage:    "xpkg.upbound.io/upbound/function-kcl-base:v0.11.2-up.1",
		transport:    http.DefaultTransport,
		imageConfigs: imageConfigs,
		upCtx:        upCtx,
	}
}
