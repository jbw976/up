// Copyright 2025 Upbound Inc.
// All rights reserved

package xpkg

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/filesystem"
)

// Layer creates a v1.Layer that represetns the layer contents for the xpkg and
// adds a corresponding label to the image Config for the layer.
func Layer(r io.Reader, fileName, annotation string, fileSize int64, mode os.FileMode, cfg *v1.Config) (v1.Layer, error) {
	tarBuf := new(bytes.Buffer)
	tw := tar.NewWriter(tarBuf)

	exHdr := &tar.Header{
		Name: fileName,
		Mode: int64(mode),
		Size: fileSize,
	}

	if err := writeLayer(tw, exHdr, r); err != nil {
		return nil, err
	}

	if err := tw.Close(); err != nil {
		return nil, errors.Wrap(err, errTarFromStream)
	}

	// TODO(hasheddan): we currently return a new reader every time here in
	// order to calculate digest, then subsequently write contents to disk. We
	// can greatly improve performance during package build by avoiding reading
	// every layer into memory.
	layer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(tarBuf.Bytes())), nil
	})
	if err != nil {
		return nil, errors.Wrap(err, errLayerFromTar)
	}

	d, err := layer.Digest()
	if err != nil {
		return nil, errors.Wrap(err, errDigestInvalid)
	}

	// add annotation label to config if a non-empty label is specified
	if annotation != "" {
		cfg.Labels[Label(d.String())] = annotation
	}

	return layer, nil
}

func writeLayer(tw *tar.Writer, hdr *tar.Header, buf io.Reader) error {
	if err := tw.WriteHeader(hdr); err != nil {
		return errors.Wrap(err, errTarFromStream)
	}

	if _, err := io.Copy(tw, buf); err != nil {
		return errors.Wrap(err, errTarFromStream)
	}
	return nil
}

// Label constructs a specially formated label using the annotationKey.
func Label(annotation string) string {
	return fmt.Sprintf("%s:%s", AnnotationKey, annotation)
}

// ImageFromFiles creates a v1.Image from arbitrary files on disk.
// Each top-level directory at `root` is a separate layer.
// The function performs no interpretation (parsing) of the files.
func ImageFromFiles(baseFs afero.Fs, root string) (v1.Image, error) {
	extManifest := empty.Image

	// We don't need detailed FileInfo or recursive traversal.
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// For now, we don't configure any special options e.g. symlink support.
		src, err := filesystem.FSToTar(afero.NewBasePathFs(baseFs, entry.Name()), entry.Name())
		if err != nil {
			return nil, err
		}

		// Create layer from the in-memory tarball
		layer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(src)), nil
		})
		if err != nil {
			return nil, err
		}
		// Append layer from the dir tarball
		extManifest, err = mutate.Append(
			extManifest,
			mutate.Addendum{
				Layer: layer,
				Annotations: map[string]string{
					AnnotationKey: entry.Name(),
				},
			},
		)
		if err != nil {
			return nil, err
		}
	}

	return extManifest, nil
}
