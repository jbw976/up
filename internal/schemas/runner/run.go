// Copyright 2025 Upbound Inc.
// All rights reserved

// Package runner contains functions for handling containers for schema generation
package runner

import (
	"context"
	"os"
	"strings"

	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/docker"
	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/internal/imageutil"
	projectv2alpha1 "github.com/upbound/up/pkg/apis/project/v2alpha1"
)

// SchemaRunner defines an interface for schema generation.
type SchemaRunner interface {
	Generate(ctx context.Context, fs afero.Fs, folder string, basePath string, imageName string, args []string, options ...Option) error
}

// RealSchemaRunner implements the SchemaRunner interface and calls runner.Generate.
type RealSchemaRunner struct {
	imageConfigs []projectv2alpha1.ImageConfig
}

// NewRealSchemaRunner for RealSchemaRunner with SchemaRunnerOption.
func NewRealSchemaRunner(opts ...ROption) *RealSchemaRunner {
	r := &RealSchemaRunner{}
	for _, o := range opts {
		o(r)
	}
	return r
}

// ROption configures the SchemaRunner.
type ROption func(*RealSchemaRunner)

// WithImageConfig adds image rewriting rules to the SchemaRunner.
func WithImageConfig(cfgs []projectv2alpha1.ImageConfig) ROption {
	return func(r *RealSchemaRunner) {
		r.imageConfigs = cfgs
	}
}

// GenerateOptions holds optional parameters for Generate.
type GenerateOptions struct {
	CopyToPath    string
	CopyFromPath  string
	WorkDirectory string
}

// Option is a function that modifies GenerateOptions.
type Option func(*GenerateOptions)

// WithCopyToPath sets the CopyToPath option.
func WithCopyToPath(path string) Option {
	return func(o *GenerateOptions) {
		o.CopyToPath = path
	}
}

// WithCopyFromPath sets the CopyFromPath option.
func WithCopyFromPath(path string) Option {
	return func(o *GenerateOptions) {
		o.CopyFromPath = path
	}
}

// WithWorkDirectory sets the WorkDirectory option.
func WithWorkDirectory(dir string) Option {
	return func(o *GenerateOptions) {
		o.WorkDirectory = dir
	}
}

// DefaultGenerateOptions provides default values.
func DefaultGenerateOptions() GenerateOptions {
	return GenerateOptions{
		CopyToPath:    "/data/input",
		CopyFromPath:  "/data/input",
		WorkDirectory: "/data/input",
	}
}

// Generate runs the containerized language tool for schema generation.
func (r RealSchemaRunner) Generate(ctx context.Context, fromFS afero.Fs, baseFolder, basePath, imageName string, command []string, options ...Option) error {
	if err := docker.Check(ctx); err != nil {
		return errors.New("failed to connect to Docker; schema generation requires a Docker-compatible container runtime")
	}

	if len(r.imageConfigs) > 0 {
		imageName = imageutil.RewriteImage(imageName, r.imageConfigs)
	}

	// Apply default options
	o := DefaultGenerateOptions()
	for _, opt := range options {
		opt(&o) // Apply each provided option
	}

	// Create the tarball from the Afero filesystem
	var opts []filesystem.FSToTarOption
	if basePath != "" {
		opts = append(opts, filesystem.WithSymlinkBasePath(basePath))
	}
	tarBuffer, err := filesystem.FSToTar(fromFS, baseFolder, opts...)
	if err != nil {
		return errors.Wrapf(err, "failed to create tar from fs")
	}

	// Fetch all environment variables starting with "UP_"
	var envVars []string
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "UP_") {
			envVars = append(envVars, e)
		}
	}

	// Create the container
	cid, err := docker.StartContainer(ctx, "", imageName,
		docker.StartWithCopyFiles(tarBuffer, o.CopyToPath),
		docker.StartWithCommand(command),
		docker.StartWithEnv(envVars...),
		docker.StartWithWorkingDirectory(o.WorkDirectory),
	)
	if err != nil {
		return err
	}

	defer func() {
		_ = docker.StopContainerByID(ctx, cid)
	}()

	// Wait for the container to finish.
	if err := docker.WaitForContainerByID(ctx, cid); err != nil {
		return err
	}

	// Copy the results back from the container to the in-memory filesystem
	if err := docker.CopyFromContainer(ctx, cid, o.CopyFromPath, fromFS); err != nil {
		return errors.Wrapf(err, "failed to copy tar from container")
	}

	return nil
}
