// Copyright 2025 Upbound Inc.
// All rights reserved

package xpkg

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/pterm/pterm"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/xpkg"
)

const (
	errMustProvideTag          = "must provide package tag if fetching from registry or daemon"
	errInvalidTag              = "package tag is not a valid reference"
	errFetchPackage            = "failed to fetch package from remote"
	errGetManifest             = "failed to get package image manifest from remote"
	errFetchLayer              = "failed to fetch annotated base layer from remote"
	errGetUncompressed         = "failed to get uncompressed contents from layer"
	errMultipleAnnotatedLayers = "package is invalid due to multiple annotated base layers"
	errOpenPackageStream       = "failed to open package stream file"
	errCreateOutputFile        = "failed to create output file"
	errCreateGzipWriter        = "failed to create gzip writer"
	errExtractPackageContents  = "failed to extract package contents"
)

const (
	layerAnnotation     = "io.crossplane.xpkg"
	baseAnnotationValue = "base"
	cacheContentExt     = ".gz"
)

// fetchFn fetches a package from a source.
type fetchFn func(context.Context, name.Reference) (v1.Image, error)

// registryFetch fetches a package from the registry.
func registryFetch(ctx context.Context, r name.Reference) (v1.Image, error) {
	return remote.Image(r, remote.WithContext(ctx))
}

// daemonFetch fetches a package from the Docker daemon.
func daemonFetch(ctx context.Context, r name.Reference) (v1.Image, error) {
	return daemon.Image(r, daemon.WithContext(ctx))
}

func xpkgFetch(path string) fetchFn {
	return func(ctx context.Context, r name.Reference) (v1.Image, error) {
		return tarball.ImageFromPath(filepath.Clean(path), nil)
	}
}

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *xpExtractCmd) AfterApply() error {
	c.fs = afero.NewOsFs()
	c.fetch = registryFetch
	if c.FromDaemon {
		c.fetch = daemonFetch
	}
	if c.FromXpkg {
		// If package is not defined, attempt to find single package in current
		// directory.
		if c.Package == "" {
			wd, err := os.Getwd()
			if err != nil {
				return errors.Wrap(err, errGetwd)
			}
			path, err := xpkg.FindXpkgInDir(c.fs, wd)
			if err != nil {
				return errors.Wrap(err, errFindPackageinWd)
			}
			c.Package = path
		}
		c.fetch = xpkgFetch(c.Package)
	}
	if !c.FromXpkg {
		if c.Package == "" {
			return errors.New(errMustProvideTag)
		}
		upCtx, err := upbound.NewFromFlags(c.Flags)
		if err != nil {
			return err
		}
		upCtx.SetupLogging()

		name, err := name.ParseReference(c.Package, name.WithDefaultRegistry(upCtx.RegistryEndpoint.Hostname()))
		if err != nil {
			return errors.Wrap(err, errInvalidTag)
		}
		c.name = name
	}
	return nil
}

// xpExtractCmd extracts package contents into a Crossplane cache compatible
// format.
type xpExtractCmd struct {
	fs    afero.Fs
	name  name.Reference
	fetch fetchFn

	Package    string `arg:""                                                                                                                                                     help:"Name of the package to extract. Must be a valid OCI image tag or a path if using --from-xpkg." optional:""`
	FromDaemon bool   `help:"Indicates that the image should be fetched from the Docker daemon."                                                                                  xor:"xp-extract-from"`
	FromXpkg   bool   `help:"Indicates that the image should be fetched from a local xpkg. If package is not specified and only one exists in current directory it will be used." xor:"xp-extract-from"`
	Output     string `default:"out.gz"                                                                                                                                           help:"Package output file path. Extension must be .gz or will be replaced."                          short:"o"`

	// Common Upbound API configuration
	Flags upbound.Flags `embed:""`
}

// Run runs the xp extract cmd.
func (c *xpExtractCmd) Run(ctx context.Context, p pterm.TextPrinter) error { //nolint:gocyclo
	// NOTE(hasheddan): most of the logic in this method is from the machinery
	// used in Crossplane's package cache and should be updated to use shared
	// libraries if moved to crossplane-runtime.

	// Fetch package.
	img, err := c.fetch(ctx, c.name)
	if err != nil {
		return errors.Wrap(err, errFetchPackage)
	}

	// Get image manifest.
	manifest, err := img.Manifest()
	if err != nil {
		return errors.Wrap(err, errGetManifest)
	}

	// Determine if the image is using annotated layers.
	var tarc io.ReadCloser
	foundAnnotated := false
	for _, l := range manifest.Layers {
		if a, ok := l.Annotations[layerAnnotation]; !ok || a != baseAnnotationValue {
			continue
		}
		// NOTE(hasheddan): the xpkg specification dictates that only one layer
		// descriptor may be annotated as xpkg base. Since iterating through all
		// descriptors is relatively inexpensive, we opt to do so in order to
		// verify that we aren't just using the first layer annotated as xpkg
		// base.
		if foundAnnotated {
			return errors.New(errMultipleAnnotatedLayers)
		}
		foundAnnotated = true
		layer, err := img.LayerByDigest(l.Digest)
		if err != nil {
			return errors.Wrap(err, errFetchLayer)
		}
		tarc, err = layer.Uncompressed()
		if err != nil {
			return errors.Wrap(err, errGetUncompressed)
		}
	}

	// If we still don't have content then we need to flatten image filesystem.
	if !foundAnnotated {
		tarc = mutate.Extract(img)
	}

	// The ReadCloser is an uncompressed tarball, either consisting of annotated
	// layer contents or flattened filesystem content. Either way, we only want
	// the package YAML stream.
	t := tar.NewReader(tarc)
	var size int64
	for {
		h, err := t.Next()
		if err != nil {
			return errors.Wrap(err, errOpenPackageStream)
		}
		if h.Name == xpkg.StreamFile {
			size = h.Size
			break
		}
	}

	out := xpkg.ReplaceExt(filepath.Clean(c.Output), cacheContentExt)
	cf, err := c.fs.Create(out)
	if err != nil {
		return errors.Wrap(err, errCreateOutputFile)
	}
	defer cf.Close() //nolint:errcheck
	w, err := gzip.NewWriterLevel(cf, gzip.BestSpeed)
	if err != nil {
		return errors.Wrap(err, errCreateGzipWriter)
	}
	if _, err = io.CopyN(w, t, size); err != nil {
		return errors.Wrap(err, errExtractPackageContents)
	}
	// NOTE(hasheddan): gzip writer must be closed to ensure all data is flushed
	// to file.
	if err := w.Close(); err != nil {
		return errors.Wrap(err, errExtractPackageContents)
	}

	p.Printfln("xpkg contents extracted to %s", out)
	return nil
}
