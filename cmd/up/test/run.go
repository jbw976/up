// Copyright 2025 Upbound Inc.
// All rights reserved

// Package test contains commands for working with tests project.
package test

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/alecthomas/kong"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1cache "github.com/google/go-containerregistry/pkg/v1/cache"
	chainsawapis "github.com/kyverno/chainsaw/pkg/apis"
	chainsawv1alpha1 "github.com/kyverno/chainsaw/pkg/apis/v1alpha1"
	chainsawchecks "github.com/kyverno/chainsaw/pkg/engine/checks"
	chainsawerrors "github.com/kyverno/chainsaw/pkg/engine/operations/errors"
	chainsawcompilers "github.com/kyverno/kyverno-json/pkg/core/compilers"
	"github.com/pterm/pterm"
	"github.com/spf13/afero"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	uptest "github.com/crossplane/uptest/pkg"

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
	"github.com/upbound/up/internal/render"
	"github.com/upbound/up/internal/test"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
	xcache "github.com/upbound/up/internal/xpkg/dep/cache"
	"github.com/upbound/up/internal/xpkg/dep/manager"
	"github.com/upbound/up/internal/xpkg/dep/resolver/image"
	"github.com/upbound/up/internal/xpkg/functions"
	"github.com/upbound/up/internal/xpkg/schemarunner"
	"github.com/upbound/up/internal/yaml"
	compositiontest "github.com/upbound/up/pkg/apis/compositiontest/v1alpha1"
	e2etest "github.com/upbound/up/pkg/apis/e2etest/v1alpha1"
	"github.com/upbound/up/pkg/apis/project/v1alpha1"
)

// runCmd is the `up test run` command.
type runCmd struct {
	Patterns               []string `arg:""                                                                                                                                     help:"The path to the test manifests"`
	ProjectFile            string   `default:"upbound.yaml"                                                                                                                     help:"Path to project definition file."                                                   short:"f"`
	Repository             string   `help:"Repository for the built package. Overrides the repository specified in the project file."                                           optional:""`
	NoBuildCache           bool     `default:"false"                                                                                                                            help:"Don't cache image layers while building."`
	BuildCacheDir          string   `default:"~/.up/build-cache"                                                                                                                help:"Path to the build cache directory."                                                 type:"path"`
	MaxConcurrency         uint     `default:"8"                                                                                                                                env:"UP_MAX_CONCURRENCY"                                                                  help:"Maximum number of functions to build and push at once."`
	ControlPlaneGroup      string   `help:"The control plane group that the control plane to use is contained in. This defaults to the group specified in the current context."`
	ControlPlaneNamePrefix string   `help:"Prefex of the control plane name to use. It will be created if not found."`
	Force                  bool     `alias:"allow-production"                                                                                                                   help:"Allow running on a control plane without the development control plane annotation." name:"skip-control-plane-check"`
	CacheDir               string   `default:"~/.up/cache/"                                                                                                                     env:"CACHE_DIR"                                                                           help:"Directory used for caching dependencies."               type:"path"`

	Kubectl string `env:"KUBECTL" help:"Absolute path to the kubectl binary. Defaults to the one in $PATH." type:"path"`

	Public bool          `help:"Create new repositories with public visibility."`
	E2E    bool          `help:"Run E2E"                                         name:"e2e"`
	Flags  upbound.Flags `embed:""`

	projFS             afero.Fs
	testFS             afero.Fs
	modelsFS           afero.Fs
	functionIdentifier functions.Identifier
	schemaRunner       schemarunner.SchemaRunner
	transport          http.RoundTripper
	m                  *manager.Manager
	r                  manager.ImageResolver
	keychain           authn.Keychain
	concurrency        uint
	proj               *v1alpha1.Project

	spaceClient  client.Client
	quiet        config.QuietFlag
	asyncWrapper async.WrapperFunc
}

func (c *runCmd) Help() string {
	return `
The 'run' command executes project tests.

Examples:
    run tests/* --e2e
        Runs all end-to-end (e2e) tests located in the 'tests/' directory.

    run tests/*
        Executes only composition tests within the 'tests/' directory.

    run tests/* --e2e --kubectl=_output/kubectl
        Runs e2e tests in 'tests/' while specifying custom paths for the Kubectl binary.
`
}

// AfterApply processes flags and sets defaults.
func (c *runCmd) AfterApply(kongCtx *kong.Context, quiet config.QuietFlag) error { //nolint: gocognit // we have multiple tests
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

	// parse the project
	proj, err := project.Parse(c.projFS, c.ProjectFile)
	if err != nil {
		return err
	}
	proj.Default()

	c.proj = proj

	c.testFS = afero.NewBasePathFs(
		c.projFS, proj.Spec.Paths.Tests,
	)

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
	c.r = r

	if c.E2E {
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

		// set the default prefix
		if c.ControlPlaneNamePrefix == "" {
			c.ControlPlaneNamePrefix = fmt.Sprintf("%s-%s", proj.Name, "uptest")
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

		tools := map[string]*string{
			"kubectl": &c.Kubectl,
		}

		for tool, path := range tools {
			if *path == "" {
				var err error
				*path, err = exec.LookPath(tool) // Updates original c.Chainsaw or c.Kubectl
				if err != nil {
					return errors.Wrapf(err, "failed to find %s in path", tool)
				}
			}
		}
	}

	pterm.EnableStyling()
	logger := logging.NewLogrLogger(zap.New(zap.UseDevMode(false)))
	kongCtx.BindTo(logger, (*logging.Logger)(nil))

	c.quiet = quiet
	c.asyncWrapper = async.WrapWithSuccessSpinners
	if quiet {
		c.asyncWrapper = async.IgnoreEvents
	}

	return nil
}

// Run is the body of the command.
func (c *runCmd) Run(ctx context.Context, upCtx *upbound.Context, log logging.Logger) error {
	upterm.DefaultObjPrinter.Pretty = true

	var err error
	var parsedTests []interface{}
	if err = upterm.WrapWithSuccessSpinner(
		"Parsing tests",
		upterm.CheckmarkSuccessSpinner,
		func() error {
			testBuilder := test.NewBuilder(
				test.BuildWithSchemaRunner(c.schemaRunner),
			)
			parsedTests, err = testBuilder.Build(ctx, c.testFS, c.Patterns, c.proj.Spec.Paths.Tests)
			if err != nil {
				return errors.Wrap(err, "failed to generate test files")
			}

			return nil
		},
		c.quiet,
	); err != nil {
		return err
	}

	if len(parsedTests) == 0 {
		pterm.Error.Println("No test files found")
		return nil
	}

	var (
		ttotal   int
		tsuccess int
		terr     int
	)

	if c.E2E {
		tests, err := e2etest.Convert(parsedTests)
		if err != nil {
			return errors.Wrap(err, "unable to validate e2e tests")
		}

		ttotal, tsuccess, terr, err = c.uptest(ctx, upCtx, tests)
		if err != nil {
			displayTestResults(ttotal, tsuccess, terr)
			return errors.Wrap(err, "unable to execute e2e tests")
		}
	} else {
		tests, err := compositiontest.Convert(parsedTests)
		if err != nil {
			return errors.Wrap(err, "unable to validate composition tests")
		}
		ttotal, tsuccess, terr, err = c.render(ctx, log, tests)
		if err != nil {
			displayTestResults(ttotal, tsuccess, terr)
			return errors.Wrap(err, "unable to execute composition tests")
		}
	}

	displayTestResults(ttotal, tsuccess, terr)
	// Return an error if there were failed tests
	if terr > 0 {
		return err
	}

	return nil
}

func (c *runCmd) render(ctx context.Context, log logging.Logger, tests []compositiontest.CompositionTest) (int, int, int, error) {
	total, success, errs := 0, 0, 0

	tempProjFS := afero.NewCopyOnWriteFs(c.projFS, afero.NewMemMapFs())

	var efns []v1.Function
	err := c.asyncWrapper(func(ch async.EventChannel) error {
		functionOptions := render.FunctionOptions{
			Project:            c.proj,
			ProjFS:             tempProjFS,
			Concurrency:        c.concurrency,
			NoBuildCache:       c.NoBuildCache,
			BuildCacheDir:      c.BuildCacheDir,
			DependecyManager:   c.m,
			FunctionIdentifier: c.functionIdentifier,
			SchemaRunner:       c.schemaRunner,
			EventChannel:       ch,
		}

		fns, err := render.BuildEmbeddedFunctionsLocalDaemon(ctx, functionOptions)
		if err != nil {
			return err
		}
		efns = fns

		return nil
	})
	if err != nil {
		return 0, 0, 0, err
	}

	var finalErr error
	for _, test := range tests {
		total++

		observedResourcesPath, err := writeToFile(tempProjFS, test.Spec.ObservedResources, "observed")
		if err != nil {
			errs++
			finalErr = errors.Join(finalErr, err)
			continue
		}

		extraResourcesPath, err := writeToFile(tempProjFS, test.Spec.ExtraResources, "extraresources")
		if err != nil {
			errs++
			finalErr = errors.Join(finalErr, err)
			continue
		}

		xrPath := test.Spec.XRPath
		if len(test.Spec.XR.Raw) > 0 {
			path, err := writeToFile(tempProjFS, []runtime.RawExtension{test.Spec.XR}, "xr")
			if err != nil {
				errs++
				finalErr = errors.Join(finalErr, err)
				continue
			}
			xrPath = path
		}

		compositionPath := test.Spec.CompositionPath
		if len(test.Spec.Composition.Raw) > 0 {
			path, err := writeToFile(tempProjFS, []runtime.RawExtension{test.Spec.Composition}, "composition")
			if err != nil {
				errs++
				finalErr = errors.Join(finalErr, err)
				continue
			}
			compositionPath = path
		}

		xrdPath := test.Spec.XRDPath
		if len(test.Spec.XRD.Raw) > 0 {
			path, err := writeToFile(tempProjFS, []runtime.RawExtension{test.Spec.XRD}, "xrd")
			if err != nil {
				errs++
				finalErr = errors.Join(finalErr, err)
				continue
			}
			xrdPath = path
		}

		options := render.Options{
			Project:                c.proj,
			ProjFS:                 tempProjFS,
			IncludeFullXR:          true,
			IncludeFunctionResults: true,
			IncludeContext:         true,
			ObservedResources:      observedResourcesPath,
			ExtraResources:         extraResourcesPath,
			CompositeResource:      xrPath,
			Composition:            compositionPath,
			XRD:                    xrdPath,
			Concurrency:            c.concurrency,
			ImageResolver:          c.r,
		}

		renderCtx, cancel := context.WithTimeout(ctx, time.Duration(test.Spec.TimeoutSeconds)*time.Second)
		defer cancel()

		output, err := render.Render(renderCtx, log, efns, options)
		if err != nil {
			errs++
			finalErr = errors.Join(finalErr, err)
			pterm.PrintOnError(err)
			continue
		}

		if err = c.asyncWrapper(func(ch async.EventChannel) error {
			eg, ctx := errgroup.WithContext(ctx)
			eg.Go(func() error {
				err = assertions(ctx, output, test.Name, test.Spec.AssertResources, ch)
				return err
			})
			return eg.Wait()
		}); err != nil {
			errs++
			finalErr = errors.Join(finalErr, err)
			continue
		}
		success++
	}

	return total, success, errs, finalErr
}

func (c *runCmd) uptest(ctx context.Context, upCtx *upbound.Context, tests []e2etest.E2ETest) (int, int, int, error) {
	var err error
	c.Repository, err = project.DetermineRepository(upCtx, c.proj, c.Repository)
	if err != nil {
		return 0, 0, 0, err
	}

	// Move the project, in memory only, to the desired repository.
	basePath := ""
	if bfs, ok := c.projFS.(*afero.BasePathFs); ok && basePath == "" {
		basePath = afero.FullBaseFsPath(bfs, ".")
	}
	c.projFS = filesystem.MemOverlay(c.projFS)

	if c.Repository != c.proj.Spec.Repository {
		if err := project.Move(ctx, c.proj, c.projFS, c.Repository); err != nil {
			return 0, 0, 0, errors.Wrap(err, "failed to update project repository")
		}
	}

	b := project.NewBuilder(
		project.BuildWithMaxConcurrency(c.concurrency),
		project.BuildWithFunctionIdentifier(c.functionIdentifier),
		project.BuildWithSchemaRunner(c.schemaRunner),
	)

	var imgMap project.ImageTagMap
	if err = c.asyncWrapper(func(ch async.EventChannel) error {
		eg, ctx := errgroup.WithContext(ctx)
		eg.Go(func() error {
			var err error
			imgMap, err = b.Build(ctx, c.proj, c.projFS,
				project.BuildWithEventChannel(ch),
				project.BuildWithImageLabels(common.ImageLabels(c)),
				project.BuildWithDependencyManager(c.m),
				project.BuildWithProjectBasePath(basePath),
			)
			return err
		})
		return eg.Wait()
	}); err != nil {
		return 0, 0, 0, err
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
	if err = c.asyncWrapper(func(ch async.EventChannel) error {
		opts := []project.PushOption{
			project.PushWithEventChannel(ch),
			project.PushWithCreatePublicRepositories(c.Public),
		}

		var err error
		generatedTag, err = pusher.Push(ctx, c.proj, imgMap, opts...)
		return err
	}); err != nil {
		return 0, 0, 0, err
	}

	total, success, errs := 0, 0, 0
	var finalErr error

	for _, test := range tests {
		total++
		err = c.executeTest(ctx, upCtx, c.proj, test, generatedTag)
		if err != nil {
			errs++
			finalErr = errors.Join(finalErr, err)
			continue
		}
		success++
	}

	return total, success, errs, finalErr
}

func writeClientConfig(clientConfig clientcmd.ClientConfig, dir string) (string, error) {
	kubeconfigPath := filepath.Join(dir, "kubeconfig.yaml")

	// Extract the raw config
	rawConfig, err := clientConfig.RawConfig()
	if err != nil {
		return "", errors.Wrap(err, "failed to get raw config from clientcmd.ClientConfig")
	}

	// Write the modified config to the file
	err = clientcmd.WriteToFile(rawConfig, kubeconfigPath)
	if err != nil {
		return "", errors.Wrap(err, "failed to write kubeconfig to file")
	}

	return kubeconfigPath, nil
}

func setEnvVars(vars map[string]string) (cleanup func(), err error) {
	for key, value := range vars {
		if err := os.Setenv(key, value); err != nil {
			return nil, errors.Wrapf(err, "failed to set environment variable %s", key)
		}
	}

	cleanup = func() {
		for key := range vars {
			if err := os.Unsetenv(key); err != nil {
				log.Printf("failed to unset environment variable %s: %v", key, err) // Logging the error
			}
		}
	}
	return cleanup, nil
}

func (c *runCmd) executeTest(ctx context.Context, upCtx *upbound.Context, proj *v1alpha1.Project, test e2etest.E2ETest, generatedTag name.Tag) error {
	// Handle OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan) // Ensure we stop receiving signals after function exits

	// Channel to signal function return
	retChan := make(chan struct{})

	controlplaneName := fmt.Sprintf("%s-%s", c.ControlPlaneNamePrefix, test.Name)

	go func() {
		select {
		case <-sigChan:
			log.Println("Received termination signal, cleaning up control plane...")
			if err := ctp.DeleteControlPlane(ctx, c.spaceClient, c.ControlPlaneGroup, controlplaneName, c.Force); err != nil {
				log.Printf("error during control plane deletion %v", err)
			}
			os.Exit(1)
		case <-retChan:
			// Function returned normally, no need for cleanup
			return
		}
	}()

	var (
		devCtpKubeconfig clientcmd.ClientConfig
		devCtpClient     client.Client
	)
	if err := c.asyncWrapper(func(ch async.EventChannel) error {
		var err error
		devCtpClient, devCtpKubeconfig, err = ctp.EnsureControlPlane(
			ctx,
			upCtx,
			c.spaceClient,
			c.ControlPlaneGroup,
			controlplaneName,
			ch,
			ctp.SkipDevCheck(c.Force),
			ctp.DevControlPlane(),
			ctp.WithCrossplaneSpec(*test.Spec.Crossplane),
		)
		return err
	}); err != nil {
		return errors.Wrap(err, "failed to create control plane")
	}

	defer func() {
		// Send signal to cleanup goroutine
		close(retChan)

		if err := ctp.DeleteControlPlane(ctx, c.spaceClient, c.ControlPlaneGroup, controlplaneName, c.Force); err != nil {
			log.Printf("error during control plane deletion %v", err)
		}
	}()

	err := c.asyncWrapper(func(ch async.EventChannel) error {
		return kube.InstallConfiguration(ctx, devCtpClient, proj.Name, generatedTag, ch)
	})
	if err != nil {
		return errors.Wrapf(err, "failed to install package")
	}

	if err := upterm.WrapWithSuccessSpinner(
		"Applying Extra Resources",
		upterm.CheckmarkSuccessSpinner,
		func() error {
			return kube.ApplyResources(ctx, devCtpClient, test.Spec.ExtraResources)
		},
		c.quiet,
	); err != nil {
		return errors.Wrap(err, "failed to apply extra resources")
	}

	tempDir, err := os.MkdirTemp("", test.Name)
	if err != nil {
		return errors.Wrap(err, "failed creating temp directory")
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			log.Printf("failed to remove temp directory %v", err)
		}
	}()

	manifestPaths := []string{}
	for i, manifest := range test.Spec.Manifests {
		if len(manifest.Raw) == 0 {
			return fmt.Errorf("manifest %d is empty", i)
		}

		manifestFile := filepath.Join(tempDir, fmt.Sprintf("manifest-%d.yaml", i))
		if err := os.WriteFile(manifestFile, manifest.Raw, 0o600); err != nil {
			return errors.Wrapf(err, "failed writing manifest %d to file", i)
		}

		manifestPaths = append(manifestPaths, manifestFile)
	}

	kubeconfigPath, err := writeClientConfig(devCtpKubeconfig, tempDir)
	if err != nil {
		return errors.Wrap(err, "error getting kubeconfig of controlplane")
	}

	vars := map[string]string{
		"KUBECTL":    c.Kubectl,
		"KUBECONFIG": kubeconfigPath,
	}

	cleanup, err := setEnvVars(vars)
	if err != nil {
		return errors.Wrap(err, "failed setting environment variables")
	}
	defer cleanup()

	builder := uptest.NewAutomatedTestBuilder()
	automatedTest := builder.
		SetManifestPaths(manifestPaths).
		SetDataSourcePath("").
		SetSetupScriptPath("").
		SetTeardownScriptPath("").
		SetDefaultConditions(test.Spec.DefaultConditions).
		SetDefaultTimeout(time.Duration(*test.Spec.TimeoutSeconds) * time.Second).
		SetDirectory(tempDir).
		SetSkipDelete(false).
		SetSkipUpdate(true).
		SetSkipImport(true).
		SetOnlyCleanUptestResources(true).
		SetRenderOnly(false).
		SetLogCollectionInterval(10 * time.Second).
		Build()

	if err := uptest.RunTest(automatedTest); err != nil {
		return errors.Wrap(err, "uptest failed")
	}

	return nil
}

func displayTestResults(ttotal, tsuccess, terr int) {
	printlnFunc := pterm.Success.Println
	if terr > 0 {
		printlnFunc = pterm.Error.Println
	}

	printlnFunc()
	printlnFunc("Tests Summary:")
	printlnFunc("------------------")
	printlnFunc("Total Tests Executed:", ttotal)
	printlnFunc("Passed tests:        ", tsuccess)
	printlnFunc("Failed tests:        ", terr)
}

func assertions(ctx context.Context, output, testName string, expectedAssertions []runtime.RawExtension, ch async.EventChannel) error {
	statusStage := fmt.Sprintf("Assert %s", testName)
	ch.SendEvent(statusStage, async.EventStatusStarted)

	// Split the rendered output into individual manifests
	manifests := parseManifests(output)
	renderedManifests := convertToUnstructured(manifests)
	expectedObjects, err := parseExpectedAssertions(expectedAssertions)
	if err != nil {
		ch.SendEvent(statusStage, async.EventStatusFailure)
		return err
	}

	assertionErrors := compareManifests(ctx, renderedManifests, expectedObjects)

	if len(assertionErrors) > 0 {
		finalErr := formatErrors(assertionErrors)
		upterm.PrintColoredError(finalErr)
		ch.SendEvent(statusStage, async.EventStatusFailure)
		return finalErr
	}

	ch.SendEvent(statusStage, async.EventStatusSuccess)

	return nil
}

func parseManifests(output string) []string {
	manifests := strings.Split(output, "---")
	parsedManifests := make([]string, 0, len(manifests))
	for _, manifest := range manifests {
		trimmed := strings.TrimSpace(manifest)
		if trimmed != "" {
			parsedManifests = append(parsedManifests, trimmed)
		}
	}
	return parsedManifests
}

func convertToUnstructured(manifests []string) []unstructured.Unstructured {
	renderedManifests := make([]unstructured.Unstructured, 0, len(manifests))
	for _, manifest := range manifests {
		var obj map[string]interface{}
		if err := yaml.Unmarshal([]byte(manifest), &obj); err != nil {
			continue
		}
		var unstructuredObj unstructured.Unstructured
		unstructuredObj.SetUnstructuredContent(obj)
		renderedManifests = append(renderedManifests, unstructuredObj)
	}
	return renderedManifests
}

func parseExpectedAssertions(expectedAssertions []runtime.RawExtension) ([]unstructured.Unstructured, error) {
	expectedObjects := make([]unstructured.Unstructured, 0, len(expectedAssertions))
	for _, assertion := range expectedAssertions {
		var expectedObj unstructured.Unstructured
		if err := json.Unmarshal(assertion.Raw, &expectedObj); err != nil {
			return nil, fmt.Errorf("failed to unmarshal expected assertion: %w", err)
		}
		expectedObjects = append(expectedObjects, expectedObj)
	}
	return expectedObjects, nil
}

func compareManifests(ctx context.Context, renderedManifests, expectedObjects []unstructured.Unstructured) []error {
	var assertionErrors []error
	for _, expected := range expectedObjects {
		if err := matchExpectedManifest(ctx, expected, renderedManifests); err != nil {
			assertionErrors = append(assertionErrors, err)
		}
	}
	return assertionErrors
}

func matchExpectedManifest(ctx context.Context, expected unstructured.Unstructured, renderedManifests []unstructured.Unstructured) error {
	expectedAPIVersion, expectedKind, expectedName := expected.GetAPIVersion(), expected.GetKind(), expected.GetName()
	expectedAnnotations := expected.GetAnnotations()

	for _, rendered := range renderedManifests {
		if isMatchingManifest(expected, rendered, expectedAnnotations) {
			checkErrs, err := chainsawchecks.Check(ctx, chainsawapis.DefaultCompilers, rendered.UnstructuredContent(), chainsawapis.NewBindings(), ptr.To(chainsawv1alpha1.NewCheck(expected.UnstructuredContent())))
			if err != nil {
				return fmt.Errorf("error during manifest check: %w", err)
			}

			if len(checkErrs) == 0 {
				return nil
			}

			return chainsawerrors.ResourceError(
				chainsawcompilers.DefaultCompilers,
				expected,
				rendered,
				false,
				chainsawapis.NewBindings(),
				checkErrs,
			)
		}
	}

	return errors.Errorf("no actual resource found: %s/%s/%s", expectedAPIVersion, expectedKind, expectedName)
}

func isMatchingManifest(expected, rendered unstructured.Unstructured, expectedAnnotations map[string]string) bool {
	if rendered.GetAPIVersion() != expected.GetAPIVersion() ||
		rendered.GetKind() != expected.GetKind() ||
		rendered.GetName() != expected.GetName() {
		return false
	}

	renderedAnnotations := rendered.GetAnnotations()
	for key, expectedValue := range expectedAnnotations {
		if renderedValue, exists := renderedAnnotations[key]; !exists || renderedValue != expectedValue {
			return false
		}
	}
	return true
}

func formatErrors(errors []error) error {
	formattedErrors := make([]string, len(errors))
	for i, e := range errors {
		formattedErrors[i] = e.Error()
	}
	return fmt.Errorf("\n%s", strings.Join(formattedErrors, "\n"))
}

func writeToFile(fs afero.Fs, resources []runtime.RawExtension, filename string) (string, error) {
	if len(resources) == 0 {
		return "", nil
	}

	// Define file path
	filePath := fmt.Sprintf("/resources/%s.yaml", filename)

	// Ensure directory exists
	if err := fs.MkdirAll("/resources", 0o755); err != nil {
		return "", err
	}

	// Open file for writing (Create or Truncate existing)
	file, err := fs.Create(filePath)
	if err != nil {
		return "", err
	}

	var content []byte
	for _, res := range resources {
		trimmed := strings.TrimSpace(string(res.Raw)) // Trim leading/trailing whitespace
		content = append(content, []byte(trimmed)...)
		content = append(content, []byte("\n---\n")...) // Ensure correct separator format
	}

	// Write content to file
	if _, err := file.Write(content); err != nil {
		return "", err
	}

	return filePath, nil
}
