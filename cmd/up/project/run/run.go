// Copyright 2025 Upbound Inc.
// All rights reserved

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
	"github.com/spf13/afero"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/scheme"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	xpkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	xpkgv1beta1 "github.com/crossplane/crossplane/apis/pkg/v1beta1"

	ctxcmd "github.com/upbound/up/cmd/up/ctx"
	"github.com/upbound/up/cmd/up/project/common"
	"github.com/upbound/up/internal/async"
	"github.com/upbound/up/internal/config"
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

// Flags are the cmd line flags specific to `up project run`. These are
// separated from Cmd struct so that they can be re-used elsewhere in the CLI.
type Flags struct {
	ProjectFile    string `default:"upbound.yaml"                                                                           help:"Path to project definition file."         short:"f"`
	Repository     string `help:"Repository for the built package. Overrides the repository specified in the project file." optional:""`
	NoBuildCache   bool   `default:"false"                                                                                  help:"Don't cache image layers while building."`
	BuildCacheDir  string `default:"~/.up/build-cache"                                                                      help:"Path to the build cache directory."       type:"path"`
	MaxConcurrency uint   `default:"8"                                                                                      env:"UP_MAX_CONCURRENCY"                        help:"Maximum number of functions to build and push at once."`
}

// Cmd is the `up project run` command.
type Cmd struct {
	Flags

	ControlPlaneGroup string        `help:"The control plane group that the control plane to use is contained in. This defaults to the group specified in the current context."`
	ControlPlaneName  string        `help:"Name of the control plane to use. It will be created if not found. Defaults to the project name."`
	Force             bool          `alias:"allow-production"                                                                                                                   help:"Allow running on a control plane without the development control plane annotation."                      name:"skip-control-plane-check"`
	CacheDir          string        `default:"~/.up/cache/"                                                                                                                     env:"CACHE_DIR"                                                                                                help:"Directory used for caching dependencies." type:"path"`
	Public            bool          `help:"Create new repositories with public visibility."`
	Timeout           time.Duration `default:"5m"                                                                                                                               help:"Maximum time to wait for the project to become ready in the control plane. Set to zero to wait forever."`
	GlobalFlags       upbound.Flags `embed:""`

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

	proj *v1alpha1.Project
}

// AfterApply processes flags and sets defaults.
func (c *Cmd) AfterApply(kongCtx *kong.Context, printer upterm.ObjectPrinter) error {
	c.concurrency = max(1, c.MaxConcurrency)

	upCtx, err := upbound.NewFromFlags(c.GlobalFlags)
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

	prj, err := project.Parse(c.projFS, c.ProjectFile)
	if err != nil {
		return errors.New("this is not a project directory")
	}
	prj.Default()
	c.proj = prj

	c.functionIdentifier = functions.DefaultIdentifier
	c.schemaRunner = schemarunner.NewRealSchemaRunner(
		schemarunner.WithImageConfig(prj.Spec.ImageConfig),
	)
	c.transport = http.DefaultTransport
	c.keychain = upCtx.RegistryKeychain()

	fs := afero.NewOsFs()
	cache, err := xcache.NewLocal(c.CacheDir, xcache.WithFS(fs))
	if err != nil {
		return err
	}
	r := image.NewResolver(
		image.WithImageConfig(prj.Spec.ImageConfig),
		image.WithFetcher(
			image.NewLocalFetcher(
				image.WithKeychain(upCtx.RegistryKeychain()),
			),
		),
	)

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

	c.quiet = printer.Quiet
	switch {
	case bool(printer.Quiet):
		c.asyncWrapper = async.IgnoreEvents
	case printer.Pretty:
		c.asyncWrapper = async.WrapWithSuccessSpinnersPretty
	default:
		c.asyncWrapper = async.WrapWithSuccessSpinnersNonPretty
	}

	return nil
}

// Run is the body of the command.
func (c *Cmd) Run(ctx context.Context, upCtx *upbound.Context) error {
	var err error
	c.Repository, err = project.DetermineRepository(upCtx, c.proj, c.Repository)
	if err != nil {
		return err
	}

	// Move the project, in memory only, to the desired repository.
	basePath := ""
	if bfs, ok := c.projFS.(*afero.BasePathFs); ok && basePath == "" {
		basePath = afero.FullBaseFsPath(bfs, ".")
	}
	c.projFS = filesystem.MemOverlay(c.projFS)

	if c.Repository != c.proj.Spec.Repository {
		if err := project.Move(ctx, c.proj, c.projFS, c.Repository); err != nil {
			return errors.Wrap(err, "failed to update project repository")
		}
	}

	if c.ControlPlaneName == "" {
		c.ControlPlaneName = c.proj.Name
	}

	b := project.NewBuilder(
		project.BuildWithMaxConcurrency(c.concurrency),
		project.BuildWithFunctionIdentifier(c.functionIdentifier),
		project.BuildWithSchemaRunner(c.schemaRunner),
	)

	var (
		imgMap project.ImageTagMap
		devCtp ctp.DevControlPlane
	)
	err = c.asyncWrapper(func(ch async.EventChannel) error {
		eg, ctx := errgroup.WithContext(ctx)

		eg.Go(func() error {
			var err error
			devCtp, err = ctp.EnsureDevControlPlane(
				ctx,
				upCtx,
				c.spaceClient,
				c.ControlPlaneGroup,
				c.ControlPlaneName,
				ch,
				ctp.SkipDevCheck(c.Force),
			)
			if err != nil {
				return err
			}

			ctpSchemeBuilders := []*scheme.Builder{
				xpkgv1.SchemeBuilder,
				xpkgv1beta1.SchemeBuilder,
			}
			for _, bld := range ctpSchemeBuilders {
				if err := bld.AddToScheme(devCtp.Client().Scheme()); err != nil {
					return err
				}
			}
			return err
		})

		eg.Go(func() error {
			var err error
			imgMap, err = b.Build(ctx, upCtx, c.proj, c.projFS,
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
		generatedTag, err = pusher.Push(ctx, c.proj, imgMap, opts...)
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

	return c.asyncWrapper(func(ch async.EventChannel) error {
		return kube.InstallConfiguration(readyCtx, devCtp.Client(), c.proj.Name, generatedTag, ch)
	})
}
