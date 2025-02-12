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

// Package run provides the `up project run` command.
package run

import (
	"context"
	"net/http"
	"path/filepath"
	"time"

	"github.com/alecthomas/kong"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1cache "github.com/google/go-containerregistry/pkg/v1/cache"
	"github.com/pterm/pterm"
	"github.com/spf13/afero"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	ctxcmd "github.com/upbound/up/cmd/up/ctx"
	"github.com/upbound/up/cmd/up/project/common"
	"github.com/upbound/up/internal/async"
	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/credhelper"
	"github.com/upbound/up/internal/ctp"
	"github.com/upbound/up/internal/ctx"
	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/internal/kube"
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

// Cmd is the `up project run` command.
type Cmd struct {
	ProjectFile       string        `default:"upbound.yaml"                                                                                                                     help:"Path to project definition file."                                                                        short:"f"`
	Repository        string        `help:"Repository for the built package. Overrides the repository specified in the project file."                                           optional:""`
	NoBuildCache      bool          `default:"false"                                                                                                                            help:"Don't cache image layers while building."`
	BuildCacheDir     string        `default:"~/.up/build-cache"                                                                                                                help:"Path to the build cache directory."                                                                      type:"path"`
	MaxConcurrency    uint          `default:"8"                                                                                                                                env:"UP_MAX_CONCURRENCY"                                                                                       help:"Maximum number of functions to build and push at once."`
	ControlPlaneGroup string        `help:"The control plane group that the control plane to use is contained in. This defaults to the group specified in the current context."`
	ControlPlaneName  string        `help:"Name of the control plane to use. It will be created if not found. Defaults to the project name."`
	Force             bool          `alias:"allow-production"                                                                                                                   help:"Allow running on a control plane without the development control plane annotation."                      name:"skip-control-plane-check"`
	CacheDir          string        `default:"~/.up/cache/"                                                                                                                     env:"CACHE_DIR"                                                                                                help:"Directory used for caching dependencies."               type:"path"`
	Public            bool          `help:"Create new repositories with public visibility."`
	Timeout           time.Duration `default:"5m"                                                                                                                               help:"Maximum time to wait for the project to become ready in the control plane. Set to zero to wait forever."`
	Flags             upbound.Flags `embed:""`

	projFS             afero.Fs
	modelsFS           afero.Fs
	functionIdentifier functions.Identifier
	schemaRunner       schemarunner.SchemaRunner
	transport          http.RoundTripper
	m                  *manager.Manager
	keychain           authn.Keychain
	concurrency        uint

	spaceClient client.Client

	quiet        config.QuietFlag
	asyncWrapper async.WrapperFunc
}

// AfterApply processes flags and sets defaults.
func (c *Cmd) AfterApply(kongCtx *kong.Context, quiet config.QuietFlag) error {
	c.concurrency = max(1, c.MaxConcurrency)

	upCtx, err := upbound.NewFromFlags(c.Flags)
	if err != nil {
		return err
	}
	upCtx.SetupLogging()
	kongCtx.Bind(upCtx)

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

	c.functionIdentifier = functions.DefaultIdentifier
	c.schemaRunner = schemarunner.RealSchemaRunner{}
	c.transport = http.DefaultTransport
	c.keychain = authn.NewMultiKeychain(
		authn.NewKeychainFromHelper(
			credhelper.New(
				credhelper.WithDomain(upCtx.Domain.Hostname()),
				credhelper.WithProfile(upCtx.ProfileName),
			),
		),
		authn.DefaultKeychain,
	)

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

	spaceCtx, err := ctx.GetCurrentSpaceNavigation(context.Background(), upCtx)
	if err != nil {
		return err
	}

	var ok bool
	var space ctxcmd.Space

	if space, ok = spaceCtx.(ctxcmd.Space); !ok {
		if group, ok := spaceCtx.(*ctxcmd.Group); ok {
			space = group.Space
			if c.ControlPlaneGroup == "" {
				c.ControlPlaneGroup = group.Name
			}
		} else if ctp, ok := spaceCtx.(*ctxcmd.ControlPlane); ok {
			space = ctp.Group.Space
			if c.ControlPlaneGroup == "" {
				c.ControlPlaneGroup = ctp.Group.Name
			}
		} else {
			return errors.New("current kubeconfig is not pointed at an Upbound Cloud Space; use `up ctx` to select a Space")
		}
	}

	// fallback to the default "default" group
	if c.ControlPlaneGroup == "" {
		c.ControlPlaneGroup = "default"
	}

	// Get the client for parent space, even if pointed at a group or a control
	// plane
	spaceClientConfig, err := space.BuildKubeconfig(types.NamespacedName{
		Namespace: c.ControlPlaneGroup,
	})
	if err != nil {
		return errors.Wrap(err, "failed to build space client")
	}
	spaceClientREST, err := spaceClientConfig.ClientConfig()
	if err != nil {
		return errors.Wrap(err, "failed to get REST config for space client")
	}
	c.spaceClient, err = client.New(spaceClientREST, client.Options{})
	if err != nil {
		return err
	}

	pterm.EnableStyling()

	c.quiet = quiet
	c.asyncWrapper = async.WrapWithSuccessSpinners
	if quiet {
		c.asyncWrapper = async.IgnoreEvents
	}

	return nil
}

// Run is the body of the command.
func (c *Cmd) Run(ctx context.Context, upCtx *upbound.Context) error {
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

	c.Repository, err = project.DetermineRepository(upCtx, proj, c.Repository)
	if err != nil {
		return err
	}

	// Move the project, in memory only, to the desired repository.
	basePath := ""
	if bfs, ok := c.projFS.(*afero.BasePathFs); ok && basePath == "" {
		basePath = afero.FullBaseFsPath(bfs, ".")
	}
	c.projFS = filesystem.MemOverlay(c.projFS)

	if c.Repository != proj.Spec.Repository {
		if err := project.Move(ctx, proj, c.projFS, c.Repository); err != nil {
			return errors.Wrap(err, "failed to update project repository")
		}
	}

	if c.ControlPlaneName == "" {
		c.ControlPlaneName = proj.Name
	}

	b := project.NewBuilder(
		project.BuildWithMaxConcurrency(c.concurrency),
		project.BuildWithFunctionIdentifier(c.functionIdentifier),
		project.BuildWithSchemaRunner(c.schemaRunner),
	)

	var (
		imgMap       project.ImageTagMap
		devCtpClient client.Client
	)
	err = c.asyncWrapper(func(ch async.EventChannel) error {
		eg, ctx := errgroup.WithContext(ctx)

		eg.Go(func() error {
			var err error
			devCtpClient, _, err = ctp.EnsureControlPlane(
				ctx,
				upCtx,
				c.spaceClient,
				c.ControlPlaneGroup,
				c.ControlPlaneName,
				ch,
				ctp.SkipDevCheck(c.Force),
				ctp.DevControlPlane(),
			)
			return err
		})

		eg.Go(func() error {
			var err error
			imgMap, err = b.Build(ctx, proj, c.projFS,
				project.BuildWithEventChannel(ch),
				project.BuildWithImageLabels(common.ImageLabels(c)),
				project.BuildWithDependencyManager(c.m),
				project.BuildWithProjectBasePath(basePath),
			)
			return err
		})

		return eg.Wait()
	})
	if err != nil {
		return err
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

	pusher := project.NewPusher(
		project.PushWithUpboundContext(upCtx),
		project.PushWithTransport(c.transport),
		project.PushWithAuthKeychain(c.keychain),
		project.PushWithMaxConcurrency(c.concurrency),
	)

	var generatedTag name.Tag
	err = c.asyncWrapper(func(ch async.EventChannel) error {
		opts := []project.PushOption{
			project.PushWithEventChannel(ch),
			project.PushWithCreatePublicRepositories(c.Public),
		}

		var err error
		generatedTag, err = pusher.Push(ctx, proj, imgMap, opts...)
		return err
	})
	if err != nil {
		return err
	}

	readyCtx := ctx
	if c.Timeout != 0 {
		timeoutCtx, cancel := context.WithTimeout(ctx, c.Timeout)
		defer cancel()
		readyCtx = timeoutCtx
	}

	err = kube.InstallConfiguration(readyCtx, devCtpClient, proj.Name, generatedTag, c.quiet)
	if err != nil {
		return err
	}

	return nil
}
