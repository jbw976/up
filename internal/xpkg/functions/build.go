// Copyright 2025 Upbound Inc.
// All rights reserved

// Package functions contains functions for building functions
package functions

import (
	"context"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	projectv1alpha1 "github.com/upbound/up/pkg/apis/project/v1alpha1"
)

const (
	errNoSuitableBuilder = "no suitable builder found"
)

// Identifier knows how to identify an appropriate builder for a function based
// on it source code.
type Identifier interface {
	// Identify returns a suitable builder for the function whose source lives
	// in the given filesystem. It returns an error if no such builder is
	// available.
	Identify(fromFS afero.Fs, imageConfigs []projectv1alpha1.ImageConfig) (Builder, error)
}

type realIdentifier struct{}

// DefaultIdentifier is the default builder identifier, suitable for production
// use.
//
//nolint:gochecknoglobals // we want to keep this global
var DefaultIdentifier = realIdentifier{}

func (realIdentifier) Identify(fromFS afero.Fs, imageConfigs []projectv1alpha1.ImageConfig) (Builder, error) {
	// builders are the known builder types, in order of precedence.
	builders := []Builder{
		newKCLBuilder(imageConfigs),
		newPythonBuilder(imageConfigs),
		newGoBuilder(imageConfigs),
		newGoTemplatingBuilder(imageConfigs),
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

func (nopIdentifier) Identify(_ afero.Fs, _ []projectv1alpha1.ImageConfig) (Builder, error) {
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
