// Copyright 2024 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package build provides the `up project build` command.
package build

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/alecthomas/kong"
	"github.com/google/go-containerregistry/pkg/name"
	v1cache "github.com/google/go-containerregistry/pkg/v1/cache"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/pterm/pterm"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/cmd/up/project/common"
	"github.com/upbound/up/internal/async"
	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/internal/oci/cache"
	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
	xcache "github.com/upbound/up/internal/xpkg/dep/cache"
	"github.com/upbound/up/internal/xpkg/dep/manager"
	"github.com/upbound/up/internal/xpkg/dep/resolver/image"
	"github.com/upbound/up/internal/xpkg/functions"
	"github.com/upbound/up/internal/xpkg/schemarunner"
	"github.com/upbound/up/pkg/apis/project/v1alpha1"
)

// Cmd is the `up project build` command.
type Cmd struct {
	ProjectFile    string        `default:"upbound.yaml"                                                                           help:"Path to project definition file."                              short:"f"`
	Repository     string        `help:"Repository for the built package. Overrides the repository specified in the project file." optional:""`
	OutputDir      string        `default:"_output"                                                                                help:"Path to the output directory, where packages will be written." short:"o"`
	NoBuildCache   bool          `default:"false"                                                                                  help:"Don't cache image layers while building."`
	BuildCacheDir  string        `default:"~/.up/build-cache"                                                                      help:"Path to the build cache directory."                            type:"path"`
	MaxConcurrency uint          `default:"8"                                                                                      env:"UP_MAX_CONCURRENCY"                                             help:"Maximum number of functions to build at once."`
	CacheDir       string        `default:"~/.up/cache/"                                                                           env:"CACHE_DIR"                                                      help:"Directory used for caching dependencies."      type:"path"`
	Flags          upbound.Flags `embed:""`

	modelsFS afero.Fs
	outputFS afero.Fs
	projFS   afero.Fs

	functionIdentifier functions.Identifier
	schemaRunner       schemarunner.SchemaRunner

	m *manager.Manager

	quiet config.QuietFlag
}

// AfterApply parses flags and applies defaults.
func (c *Cmd) AfterApply(kongCtx *kong.Context, quiet config.QuietFlag) error {
	upCtx, err := upbound.NewFromFlags(c.Flags)
	if err != nil {
		return err
	}
	upCtx.SetupLogging()
	kongCtx.Bind(upCtx)

	kongCtx.Bind(pterm.DefaultBulletList.WithWriter(kongCtx.Stdout))
	ctx := context.Background()
	// Read the project file.
	projFilePath, err := filepath.Abs(c.ProjectFile)
	if err != nil {
		return err
	}
	// The location of the project file defines the root of the project.
	projDirPath := filepath.Dir(projFilePath)
	// Construct a virtual filesystem that contains only the project. We'll do
	// all our operations inside this virtual FS.
	c.projFS = afero.NewBasePathFs(afero.NewOsFs(), projDirPath)
	c.modelsFS = afero.NewBasePathFs(afero.NewOsFs(), filepath.Join(projDirPath, ".up"))

	// Output can be anywhere, doesn't have to be in the project directory.
	c.outputFS = afero.NewOsFs()
	fs := afero.NewOsFs()

	cache, err := xcache.NewLocal(c.CacheDir, xcache.WithFS(fs))
	if err != nil {
		return err
	}

	r := image.NewResolver()

	m, err := manager.New(
		manager.WithCacheModels(c.modelsFS),
		manager.WithCache(cache),
		manager.WithSkipCacheUpdateIfExists(true),
		manager.WithResolver(r),
	)
	if err != nil {
		return err
	}

	c.m = m

	c.functionIdentifier = functions.DefaultIdentifier
	c.schemaRunner = schemarunner.RealSchemaRunner{}

	// workaround interfaces not being bindable ref: https://github.com/alecthomas/kong/issues/48
	kongCtx.BindTo(ctx, (*context.Context)(nil))

	c.quiet = quiet
	return nil
}

// Run runs the command.
func (c *Cmd) Run(ctx context.Context, upCtx *upbound.Context) error { //nolint:gocyclo // This is fine.
	pterm.EnableStyling()

	if c.MaxConcurrency == 0 {
		c.MaxConcurrency = 1
	}

	var proj *v1alpha1.Project
	err := upterm.WrapWithSuccessSpinner(
		"Parsing project metadata",
		upterm.CheckmarkSuccessSpinner,
		func() error {
			projFilePath := filepath.Join("/", filepath.Base(c.ProjectFile))
			lproj, err := project.Parse(c.projFS, projFilePath)
			if err != nil {
				return errors.Wrap(err, "failed to parse project metadata")
			}
			lproj.Default()
			proj = lproj
			return nil
		},
		c.quiet,
	)
	if err != nil {
		return err
	}

	basePath := ""
	if c.Repository != "" {
		// Update the project (in-memory) to use the new repository. This
		// ensures function references in compositions are consistent with the
		// project's repository.
		ref, err := name.NewRepository(c.Repository, name.WithDefaultRegistry(upCtx.RegistryEndpoint.Host))
		if err != nil {
			return errors.Wrap(err, "failed to parse repository")
		}

		if bfs, ok := c.projFS.(*afero.BasePathFs); ok && basePath == "" {
			basePath = afero.FullBaseFsPath(bfs, ".")
		}
		c.projFS = filesystem.MemOverlay(c.projFS)
		if err := project.Move(ctx, proj, c.projFS, ref.String()); err != nil {
			return errors.Wrap(err, "failed to update project repository")
		}
	}

	b := project.NewBuilder(
		project.BuildWithMaxConcurrency(c.MaxConcurrency),
		project.BuildWithFunctionIdentifier(c.functionIdentifier),
		project.BuildWithSchemaRunner(c.schemaRunner),
	)

	var imgMap project.ImageTagMap
	err = async.WrapWithSuccessSpinners(func(ch async.EventChannel) error {
		var err error
		imgMap, err = b.Build(ctx, proj, c.projFS,
			project.BuildWithEventChannel(ch, c.quiet),
			project.BuildWithImageLabels(common.ImageLabels(c)),
			project.BuildWithDependencyManager(c.m),
			project.BuildWithProjectBasePath(basePath),
		)
		return err
	})
	if err != nil {
		return err
	}

	outFile := filepath.Join(c.OutputDir, fmt.Sprintf("%s.uppkg", proj.Name))
	err = c.outputFS.MkdirAll(c.OutputDir, 0o755)
	if err != nil {
		return errors.Wrapf(err, "failed to create output directory %q", c.OutputDir)
	}

	if !c.NoBuildCache {
		// Create a layer cache so that if we're building on top of base images we
		// only pull their layers once. Note we do this here rather than in the
		// builder because pulling layers is deferred to where we use them, which is
		// here.
		cch := cache.NewValidatingCache(v1cache.NewFilesystemCache(c.BuildCacheDir))
		for tag, img := range imgMap {
			imgMap[tag] = v1cache.Image(img, cch)
		}
	}

	err = upterm.WrapWithSuccessSpinner(
		fmt.Sprintf("Writing packages to %s", outFile),
		upterm.CheckmarkSuccessSpinner,
		func() error {
			f, err := c.outputFS.Create(outFile)
			if err != nil {
				return errors.Wrapf(err, "failed to create output file %q", outFile)
			}
			defer f.Close() //nolint:errcheck // Can't do anything useful with this error.

			err = tarball.MultiWrite(imgMap, f)
			if err != nil {
				return errors.Wrap(err, "failed to write package to file")
			}
			return nil
		},
		c.quiet,
	)
	if err != nil {
		return err
	}

	return nil
}
