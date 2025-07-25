// Copyright 2025 Upbound Inc.
// All rights reserved

// Package xpkg contains commands for building and pushing packages. These
// commands are deprecated.
package xpkg

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/pterm/pterm"
	"github.com/spf13/afero"
	"golang.org/x/sync/errgroup"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up-sdk-go/service/repositories"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/xpkg"
)

const (
	errCreateNotUpbound  = "cannot create repository for non-Upbound registry"
	errCreateAccountRepo = "cannot create repository without account and repository names"
	errCreateRepo        = "failed to create repository"
	errGetwd             = "failed to get working directory while searching for package"
	errFindPackageinWd   = "failed to find a package in current working directory"
	errBuildImage        = "failed to build image from layers"
)

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *pushCmd) AfterApply(kongCtx *kong.Context) error {
	c.fs = afero.NewOsFs()
	upCtx, err := upbound.NewFromFlags(c.Flags)
	if err != nil {
		return err
	}
	upCtx.SetupLogging()

	kongCtx.Bind(upCtx)

	return nil
}

// pushCmd pushes a package.
type pushCmd struct {
	fs afero.Fs

	Tag     string   `arg:""                                                                                                      help:"Tag of the package to be pushed. Must be a valid OCI image tag."`
	Package []string `help:"Path to packages. If not specified and only one package exists in current directory it will be used." short:"f"`
	Create  bool     `help:"Create repository on push if it does not exist."`

	// Common Upbound API configuration
	Flags upbound.Flags `embed:""`
}

// Run runs the push cmd.
func (c *pushCmd) Run(p pterm.TextPrinter, upCtx *upbound.Context) error {
	// If package is not defined, attempt to find single package in current
	// directory.
	if len(c.Package) == 0 {
		wd, err := os.Getwd()
		if err != nil {
			return errors.Wrap(err, errGetwd)
		}
		path, err := xpkg.FindXpkgInDir(c.fs, wd)
		if err != nil {
			return errors.Wrap(err, errFindPackageinWd)
		}
		c.Package = []string{path}
	}

	imgs := make([]v1.Image, 0, len(c.Package))
	for _, p := range c.Package {
		img, err := tarball.ImageFromPath(filepath.Clean(p), nil)
		if err != nil {
			return err
		}
		imgs = append(imgs, img)
	}
	return PushImages(p, upCtx, imgs, c.Tag, c.Create, c.Flags.Profile)
}

// PushImages pushes images.
func PushImages(p pterm.TextPrinter, upCtx *upbound.Context, imgs []v1.Image, t string, create bool, _ string) error { //nolint:gocognit // We will delete this soon.
	tag, err := name.NewTag(t, name.WithDefaultRegistry(upCtx.RegistryEndpoint.Hostname()))
	if err != nil {
		return err
	}

	kc := upCtx.RegistryKeychain()

	if create {
		if !strings.Contains(tag.RegistryStr(), upCtx.RegistryEndpoint.Hostname()) {
			return errors.New(errCreateNotUpbound)
		}
		parts := strings.Split(tag.RepositoryStr(), "/")
		if len(parts) != 2 {
			return errors.New(errCreateAccountRepo)
		}
		cfg, err := upCtx.BuildSDKConfig()
		if err != nil {
			return err
		}
		if err := repositories.NewClient(cfg).CreateOrUpdate(context.Background(), parts[0], parts[1]); err != nil { //nolint:staticcheck // This file will be deleted soon.
			return errors.Wrap(err, errCreateRepo)
		}
	}

	adds := make([]mutate.IndexAddendum, len(imgs))

	// NOTE(hasheddan): the errgroup context is passed to each image write,
	// meaning that if one fails it will cancel others that are in progress.
	g, ctx := errgroup.WithContext(context.Background())
	for i, img := range imgs {
		g.Go(func() error {
			// annotate image layers
			aimg, err := xpkg.AnnotateImage(img)
			if err != nil {
				return err
			}

			var t name.Reference = tag
			if len(imgs) > 1 {
				d, err := aimg.Digest()
				if err != nil {
					return err
				}
				t, err = name.NewDigest(fmt.Sprintf("%s@%s", tag.Repository.Name(), d.String()), name.WithDefaultRegistry(upCtx.RegistryEndpoint.Hostname()))
				if err != nil {
					return err
				}

				mt, err := aimg.MediaType()
				if err != nil {
					return err
				}

				conf, err := aimg.ConfigFile()
				if err != nil {
					return err
				}

				adds[i] = mutate.IndexAddendum{
					Add: aimg,
					Descriptor: v1.Descriptor{
						MediaType: mt,
						Platform: &v1.Platform{
							Architecture: conf.Architecture,
							OS:           conf.OS,
							OSVersion:    conf.OSVersion,
						},
					},
				}
			}
			if err := remote.Write(t, aimg, remote.WithAuthFromKeychain(kc), remote.WithContext(ctx)); err != nil {
				return err
			}
			return nil
		})
	}

	// Error if writing any images failed.
	if err := g.Wait(); err != nil {
		return err
	}

	// If we pushed more than one xpkg then we need to write index.
	if len(imgs) > 1 {
		if err := remote.WriteIndex(tag, mutate.AppendManifests(empty.Index, adds...), remote.WithAuthFromKeychain(kc)); err != nil {
			return err
		}
	}

	p.Printfln("xpkg pushed to %s", tag.String())
	return nil
}
