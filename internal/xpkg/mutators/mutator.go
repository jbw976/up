// Copyright 2025 Upbound Inc.
// All rights reserved

package mutators

import (
	"bytes"
	"io"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/xpkg"
	"github.com/upbound/up/internal/xpkg/parser/schema"
)

// SchemaMutator is responsible for generating and adding a Schema layer.
type SchemaMutator struct {
	sk            *schema.Parser
	annotationKey string
}

// NewSchemaMutator creates a new SchemaMutator.
func NewSchemaMutator(sk *schema.Parser, annotationKey string) *SchemaMutator {
	return &SchemaMutator{
		sk:            sk,
		annotationKey: annotationKey,
	}
}

// Mutate generates and adds the Schema layer to the given image and config.
func (m *SchemaMutator) Mutate(img v1.Image, cfg v1.Config) (v1.Image, v1.Config, error) {
	if m.sk == nil || m.sk.Filesystem == nil {
		return img, cfg, nil // No mutation if Schema parser or filesystem is missing.
	}

	// Initialize the Pparser with the file system, root path, and file mode.
	schemaParser := schema.New(m.sk.Filesystem, "", xpkg.StreamFileMode)

	// Generate the tarball using the Parser
	schemaTarball, err := schemaParser.Generate()
	if err != nil {
		return nil, cfg, errors.Wrap(err, "failed to generate Schema tarball")
	}

	// Convert the tarball to a v1.Layer.
	schemaLayer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(schemaTarball)), nil
	})
	if err != nil {
		return nil, cfg, errors.Wrap(err, "failed to convert tarball to v1.Layer")
	}

	// Calculate the layer digest.
	layerDigest, err := schemaLayer.Digest()
	if err != nil {
		return nil, cfg, errors.Wrap(err, "failed to calculate layer digest")
	}

	// Update the image config with the annotation label.
	labelKey := xpkg.Label(layerDigest.String())
	cfg.Labels[labelKey] = m.annotationKey

	// Append the Schema layer to the image.
	img, err = mutate.AppendLayers(img, schemaLayer)
	if err != nil {
		return nil, cfg, errors.Wrap(err, "failed to append Schema layer to image")
	}

	return img, cfg, nil
}
