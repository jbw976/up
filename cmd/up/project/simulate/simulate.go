// Copyright 2025 Upbound Inc.
// All rights reserved

// Package simulate provides the `up project simulate` command.
package simulate

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
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
	"sigs.k8s.io/controller-runtime/pkg/scheme"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	xpkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	xpkgv1beta1 "github.com/crossplane/crossplane/apis/pkg/v1beta1"

	ctxcmd "github.com/upbound/up/cmd/up/ctx"
	"github.com/upbound/up/cmd/up/project/common"
	runcmd "github.com/upbound/up/cmd/up/project/run"
	"github.com/upbound/up/internal/async"
	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/ctx"
	"github.com/upbound/up/internal/diff"
	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/oci/cache"
	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/simulation"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
	xcache "github.com/upbound/up/internal/xpkg/dep/cache"
	"github.com/upbound/up/internal/xpkg/dep/manager"
	"github.com/upbound/up/internal/xpkg/dep/resolver/image"
	"github.com/upbound/up/internal/xpkg/functions"
	"github.com/upbound/up/internal/xpkg/schemarunner"
	"github.com/upbound/up/pkg/apis/project/v1alpha1"
)

// Cmd is the `up project simulate` command.
type Cmd struct {
	runcmd.Flags

	SourceControlPlaneName string `arg:"" help:"Name of the source control plane"`
	SimulationName         string `help:"The name of the simulation resource" short:"n" optional:""`

	Output            string `help:"Output the results of the simulation to the provided file. Defaults to standard out if not specified" short:"o"`
	Wait              bool   `default:"true"                                                                                              help:"Wait for the simulation to complete. If set to false, the command will exit immediately after the changeset is applied"`
	TerminateOnFinish bool   `default:"true"                                                                                             help:"Terminate the simulation after the completion criteria is met"`

	ControlPlaneGroup string        `short:"g" help:"The control plane group that the control plane to use is contained in. This defaults to the group specified in the current context."`
	CacheDir          string        `default:"~/.up/cache/"                                                                                                                     env:"CACHE_DIR"                                                                                                help:"Directory used for caching dependencies."               type:"path"`
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
}

// AfterApply processes flags and sets defaults.
func (c *Cmd) AfterApply(kongCtx *kong.Context, quiet config.QuietFlag) error {
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

	c.functionIdentifier = functions.DefaultIdentifier
	c.schemaRunner = schemarunner.RealSchemaRunner{}
	c.transport = http.DefaultTransport
	c.keychain = upCtx.RegistryKeychain()

	fs := afero.NewOsFs()
	cache, err := xcache.NewLocal(c.CacheDir, xcache.WithFS(fs))
	if err != nil {
		return err
	}
	r := image.NewResolver(
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

	pterm.EnableStyling()

	c.quiet = quiet
	c.asyncWrapper = async.WrapWithSuccessSpinners
	if quiet {
		c.asyncWrapper = async.IgnoreEvents
	}

	return nil
}

// Run is the body of the command.
func (c *Cmd) Run(ctx context.Context, upCtx *upbound.Context, kongCtx *kong.Context) error {
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

	simOpts := []simulation.Option{}

	if c.SimulationName != "" {
		simOpts = append(simOpts, simulation.WithName(c.SimulationName))
	}

	sim, err := simulation.Start(ctx, c.spaceClient, types.NamespacedName{
		Namespace: c.ControlPlaneGroup,
		Name:      c.SourceControlPlaneName,
	}, simOpts...)
	if err != nil {
		return errors.Wrap(err, "failed to start simulation")
	}

	b := project.NewBuilder(
		project.BuildWithMaxConcurrency(c.concurrency),
		project.BuildWithFunctionIdentifier(c.functionIdentifier),
		project.BuildWithSchemaRunner(c.schemaRunner),
	)

	var imgMap project.ImageTagMap
	err = c.asyncWrapper(func(ch async.EventChannel) error {
		eg, ctx := errgroup.WithContext(ctx)

		eg.Go(func() error {
			stageStatus := "Waiting for Simulation to begin accepting changes"
			ch.SendEvent(stageStatus, async.EventStatusStarted)
			err := sim.WaitForCondition(ctx, c.spaceClient, simulation.AcceptingChanges())
			if err != nil {
				ch.SendEvent(stageStatus, async.EventStatusFailure)
			} else {
				ch.SendEvent(stageStatus, async.EventStatusSuccess)
			}
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

	simConfig, err := sim.RESTConfig(ctx, upCtx)
	if err != nil {
		return errors.Wrap(err, "failed to get simulated control plane kubeconfig")
	}
	simClient, err := client.New(simConfig, client.Options{})
	if err != nil {
		return errors.Wrap(err, "failed to build simulated control plane client")
	}

	ctpSchemeBuilders := []*scheme.Builder{
		xpkgv1.SchemeBuilder,
		xpkgv1beta1.SchemeBuilder,
	}
	for _, bld := range ctpSchemeBuilders {
		if err := bld.AddToScheme(simClient.Scheme()); err != nil {
			return err
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

	err = c.asyncWrapper(func(ch async.EventChannel) error {
		return kube.InstallConfiguration(readyCtx, simClient, proj.Name, generatedTag, ch)
	})
	if err != nil {
		return err
	}

	err = c.asyncWrapper(func(ch async.EventChannel) error {
		eg, _ := errgroup.WithContext(ctx)

		eg.Go(func() error {
			stageStatus := "Simulating changes"
			ch.SendEvent(stageStatus, async.EventStatusStarted)
			time.Sleep(1 * time.Minute)
			ch.SendEvent(stageStatus, async.EventStatusSuccess)
			return err
		})

		eg.Go(func() error {
			// query for changes
			return nil
		})

		return eg.Wait()
	})
	if err != nil {
		return err
	}

	if err := sim.Complete(ctx, c.spaceClient); err != nil {
		return err
	}

	err = c.asyncWrapper(func(ch async.EventChannel) error {
		stageStatus := "Waiting for Simulation to complete"
		ch.SendEvent(stageStatus, async.EventStatusStarted)
		err := sim.WaitForCondition(ctx, c.spaceClient, simulation.Complete())
		if err != nil {
			ch.SendEvent(stageStatus, async.EventStatusFailure)
		} else {
			ch.SendEvent(stageStatus, async.EventStatusSuccess)
		}
		return err
	})
	if err != nil {
		return err
	}

	diffSet, err := sim.DiffSet(ctx, upCtx)
	if err != nil {
		return err
	}

	buf := &strings.Builder{}
	writer := diff.NewPrettyPrintWriter(buf, true)
	_ = writer.Write(diffSet)

	if _, err := fmt.Fprintf(kongCtx.Stdout, "\n\n"); err != nil {
		return errors.Wrap(err, "failed to write output")
	}
	if _, err := fmt.Fprint(kongCtx.Stdout, buf.String()); err != nil {
		return errors.Wrap(err, "failed to write output")
	}

	return nil
}
