// Copyright 2025 Upbound Inc.
// All rights reserved

// Package processor provides functions for post-processing support bundles.
package processor

import (
	"context"
	"os"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/upbound/up/internal/supportbundle/archive"
)

// Func is a function that processes an extracted bundle.
// It receives the bundle root directory (e.g., /tmp/extract/support-bundle-TIMESTAMP)
// and modifies it in place.
type Func func(ctx context.Context, bundleRoot string) error

// Apply extracts a support bundle, applies all processors in sequence,
// and repackages.
func Apply(ctx context.Context, archivePath string, processors ...Func) error {
	if len(processors) == 0 {
		return nil
	}

	tempDir, err := os.MkdirTemp("", "support-bundle-process-*")
	if err != nil {
		return errors.Wrap(err, "failed to create temp directory")
	}
	defer os.RemoveAll(tempDir) //nolint:errcheck // processing could have succeeded

	if err := archive.Extract(archivePath, tempDir); err != nil {
		return errors.Wrap(err, "failed to extract bundle")
	}

	bundleRoot, err := archive.FindBundleRoot(tempDir)
	if err != nil {
		return errors.Wrap(err, "failed to find bundle root directory")
	}

	for _, processor := range processors {
		if err := processor(ctx, bundleRoot); err != nil {
			return errors.Wrap(err, "processor failed")
		}
	}

	if err := archive.Repackage(bundleRoot, archivePath); err != nil {
		return errors.Wrap(err, "failed to repackage bundle")
	}

	return nil
}
