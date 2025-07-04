// Copyright 2025 Upbound Inc.
// All rights reserved

// Package run provides the `up project run` command.
package run

import (
	"context"
	"fmt"
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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/scheme"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	xpkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	xpkgv1beta1 "github.com/crossplane/crossplane/apis/pkg/v1beta1"

	"github.com/upbound/up/cmd/up/project/common"
	"github.com/upbound/up/internal/async"
	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/ctp"
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

	ControlPlaneGroup  string        `help:"The control plane group that the control plane to use is contained in. This defaults to the group specified in the current context."`
	ControlPlaneName   string        `help:"Name of the control plane to use. It will be created if not found. Defaults to the project name."`
	Force              bool          `alias:"allow-production"                                                                                                                   help:"Allow running on a non-development control plane."                                                       name:"skip-control-plane-check"`
	Local              bool          `help:"Use a local dev control plane, even if Spaces is available."`
	NoUpdateKubeconfig bool          `help:"Do not update kubeconfig to use the dev control plane as its current context."`
	UseCurrentContext  bool          `help:"Run the project with the current kubeconfig context rather than creating a new dev control plane."`
	CacheDir           string        `default:"~/.up/cache/"                                                                                                                     env:"CACHE_DIR"                                                                                                help:"Directory used for caching dependencies." type:"path"`
	Public             bool          `help:"Create new repositories with public visibility."`
	Timeout            time.Duration `default:"5m"                                                                                                                               help:"Maximum time to wait for the project to become ready in the control plane. Set to zero to wait forever."`
	GlobalFlags        upbound.Flags `embed:""`

	projFS             afero.Fs
	modelsFS           afero.Fs
	functionIdentifier functions.Identifier
	schemaRunner       schemarunner.SchemaRunner
	transport          http.RoundTripper
	m                  *manager.Manager
	keychain           authn.Keychain
	concurrency        uint
	pusher             project.Pusher

	// Allow these functions to be injected for testing purposes.
	ensureDevControlPlane func(context.Context, *upbound.Context, ...ctp.EnsureDevControlPlaneOption) (ctp.DevControlPlane, error)
	installConfiguration  func(context.Context, client.Client, string, name.Tag, async.EventChannel) error

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
	c.pusher = project.NewPusher(
		project.PushWithUpboundContext(upCtx),
		project.PushWithTransport(c.transport),
		project.PushWithAuthKeychain(c.keychain),
		project.PushWithMaxConcurrency(c.concurrency),
	)
	c.ensureDevControlPlane = ctp.EnsureDevControlPlane
	c.installConfiguration = kube.InstallConfiguration

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
func (c *Cmd) Run(ctx context.Context, upCtx *upbound.Context, printer pterm.TextPrinter) error { //nolint:gocognit // This could be refactored a bit, but isn't too bad.
	if c.UseCurrentContext && !c.Force {
		if err := c.confirmUseCurrentContext(upCtx); err != nil {
			return err
		}
	}

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
			if c.UseCurrentContext {
				devCtp, err = ctp.NewKubeconfigDevControlPlane(upCtx)
			} else {
				devCtp, err = c.ensureDevControlPlane(
					ctx,
					upCtx,
					ctp.WithEventChannel(ch),
					ctp.WithSpacesGroup(c.ControlPlaneGroup),
					ctp.WithControlPlaneName(c.ControlPlaneName),
					ctp.SkipDevCheck(c.Force),
					ctp.ForceLocal(c.Local),
				)
			}
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

	var generatedTag name.Tag
	err = c.asyncWrapper(func(ch async.EventChannel) error {
		opts := []project.PushOption{
			project.PushWithEventChannel(ch),
			project.PushWithCreatePublicRepositories(c.Public),
		}

		var err error
		generatedTag, err = c.pusher.Push(ctx, c.proj, imgMap, opts...)
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

	if err := c.asyncWrapper(func(ch async.EventChannel) error {
		return c.installConfiguration(readyCtx, devCtp.Client(), c.proj.Name, generatedTag, ch)
	}); err != nil {
		return err
	}

	printer.Println(devCtp.Info())

	if !c.UseCurrentContext && !c.NoUpdateKubeconfig {
		ctpKubeconfig, err := devCtp.Kubeconfig().RawConfig()
		if err != nil {
			return err
		}

		w := kube.NewFileWriter(upCtx, c.GlobalFlags.Kube.Kubeconfig, ctpKubeconfig.CurrentContext)
		if err := w.Write(&ctpKubeconfig); err != nil {
			return err
		}
		pterm.Printfln("Kubeconfig updated. Current context is %q.", ctpKubeconfig.CurrentContext)
	}

	return nil
}

const useCurrentContextConfirmFmt = `Running a project on an existing control plane can be destructive.
Are you sure you want to use kubeconfig context %q?`

func (c *Cmd) confirmUseCurrentContext(upCtx *upbound.Context) error {
	ctxName, err := upCtx.GetCurrentContextName()
	if err != nil {
		return err
	}

	confirm := pterm.DefaultInteractiveConfirm
	proceed, err := confirm.Show(fmt.Sprintf(useCurrentContextConfirmFmt, ctxName))
	if err != nil {
		return err
	}
	if !proceed {
		return errors.New("operation canceled")
	}

	return nil
}
