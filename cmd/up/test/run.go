// Copyright 2025 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package test contains commands for working with tests project.
package test

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/alecthomas/kong"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1cache "github.com/google/go-containerregistry/pkg/v1/cache"
	"github.com/pterm/pterm"
	"github.com/spf13/afero"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	uptest "github.com/crossplane/uptest/pkg"

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
	"github.com/upbound/up/internal/test"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
	xcache "github.com/upbound/up/internal/xpkg/dep/cache"
	"github.com/upbound/up/internal/xpkg/dep/manager"
	"github.com/upbound/up/internal/xpkg/dep/resolver/image"
	"github.com/upbound/up/internal/xpkg/functions"
	"github.com/upbound/up/internal/xpkg/schemarunner"
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

	Chainsaw string `env:"CHAINSAW" help:"Absolute path to the chainsaw binary. Defaults to the one in $PATH." type:"path"`
	Kubectl  string `env:"KUBECTL"  help:"Absolute path to the kubectl binary. Defaults to the one in $PATH."  type:"path"`

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
	keychain           authn.Keychain
	concurrency        uint
	proj               *v1alpha1.Project

	spaceClient client.Client
	quiet       config.QuietFlag
}

func (c *runCmd) Help() string {
	return `
The 'run' command executes project tests.

Examples:
    run tests/ --e2e
        Runs all end-to-end (e2e) tests located in the 'tests/' directory.

    run tests/
        Executes only composition tests within the 'tests/' directory.

    run tests/ --e2e --chainsaw=_output/chainsaw --kubectl=_output/kubectl
        Runs e2e tests in 'tests/' while specifying custom paths for the Chainsaw and Kubectl binaries.
`
}

// AfterApply processes flags and sets defaults.
func (c *runCmd) AfterApply(kongCtx *kong.Context, quiet config.QuietFlag) error {
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
		"chainsaw": &c.Chainsaw,
		"kubectl":  &c.Kubectl,
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

	pterm.EnableStyling()

	c.quiet = quiet

	return nil
}

// Run is the body of the command.
func (c *runCmd) Run(ctx context.Context, upCtx *upbound.Context) error {
	upterm.DefaultObjPrinter.Pretty = true

	var err error
	var parsedE2ETests []e2etest.E2ETest
	if err = upterm.WrapWithSuccessSpinner(
		"Parsing tests",
		upterm.CheckmarkSuccessSpinner,
		func() error {
			testBuilder := test.NewBuilder(
				test.BuildWithSchemaRunner(c.schemaRunner),
			)
			parsedTests, err := testBuilder.Build(ctx, c.testFS, c.Patterns, c.proj.Spec.Paths.Tests)
			if err != nil {
				return errors.Wrap(err, "failed to generate test files")
			}
			parsedE2ETests = parsedTests
			return nil
		},
		c.quiet,
	); err != nil {
		return err
	}

	if len(parsedE2ETests) == 0 {
		pterm.Error.Println("No test files found")
		return nil
	}

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

	b := project.NewBuilder(
		project.BuildWithMaxConcurrency(c.concurrency),
		project.BuildWithFunctionIdentifier(c.functionIdentifier),
		project.BuildWithSchemaRunner(c.schemaRunner),
	)

	var imgMap project.ImageTagMap
	if err = async.WrapWithSuccessSpinners(func(ch async.EventChannel) error {
		eg, ctx := errgroup.WithContext(ctx)
		eg.Go(func() error {
			var err error
			imgMap, err = b.Build(ctx, c.proj, c.projFS,
				project.BuildWithEventChannel(ch, c.quiet),
				project.BuildWithImageLabels(common.ImageLabels(c)),
				project.BuildWithDependencyManager(c.m),
				project.BuildWithProjectBasePath(basePath),
			)
			return err
		})
		return eg.Wait()
	}); err != nil {
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
	if err = async.WrapWithSuccessSpinners(func(ch async.EventChannel) error {
		opts := []project.PushOption{
			project.PushWithEventChannel(ch),
			project.PushWithCreatePublicRepositories(c.Public),
		}

		var err error
		generatedTag, err = pusher.Push(ctx, c.proj, imgMap, opts...)
		return err
	}); err != nil {
		return err
	}

	total, success, errors := 0, 0, 0
	for _, test := range parsedE2ETests {
		total++
		err := c.executeTest(ctx, upCtx, c.proj, test, generatedTag)
		if err != nil {
			errors++
			continue // Continue to the next test instead of stopping the loop
		}
		success++
	}

	printlnFunc := pterm.Success.Println
	if errors > 0 {
		printlnFunc = pterm.Error.Println
	}

	printlnFunc()
	printlnFunc("Tests Summary:")
	printlnFunc("------------------")
	printlnFunc("Total Tests Executed:", total)
	printlnFunc("Passed tests:        ", success)
	printlnFunc("Failed tests:        ", errors)

	// Return an error if there were failed tests
	if errors > 0 {
		return fmt.Errorf("%d tests failed", errors)
	}
	return nil
}

// ToDo(haarchri): use something better.
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
	if err := async.WrapWithSuccessSpinners(func(ch async.EventChannel) error {
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

	if err := kube.InstallConfiguration(ctx, devCtpClient, proj.Name, generatedTag, c.quiet); err != nil {
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
		"CHAINSAW":   c.Chainsaw,
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
