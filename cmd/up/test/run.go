// Copyright 2025 Upbound Inc.
// All rights reserved

package test

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	chainsawapis "github.com/kyverno/chainsaw/pkg/apis"
	chainsawv1alpha1 "github.com/kyverno/chainsaw/pkg/apis/v1alpha1"
	chainsawchecks "github.com/kyverno/chainsaw/pkg/engine/checks"
	chainsawerrors "github.com/kyverno/chainsaw/pkg/engine/operations/errors"
	chainsawcompilers "github.com/kyverno/kyverno-json/pkg/core/compilers"
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	"github.com/upbound/up/internal/async"
	"github.com/upbound/up/internal/ctp"
	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/schemas/runner"
	"github.com/upbound/up/internal/style"
	"github.com/upbound/up/internal/test"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/internal/xpkg/dep/manager"
	"github.com/upbound/up/internal/xpkg/dep/resolver/image"
	"github.com/upbound/up/internal/xpkg/functions"
	"github.com/upbound/up/internal/yaml"
	"github.com/upbound/up/pkg/apis"
	compositiontest "github.com/upbound/up/pkg/apis/compositiontest/v1alpha1"
	e2etest "github.com/upbound/up/pkg/apis/e2etest/v1alpha1"
	operationtest "github.com/upbound/up/pkg/apis/operationtest/v1alpha1"

	_ "embed"
)

// runCmd is the `up test run` command.
type runCmd struct {
	Patterns                []string `arg:""                                                                                                                                     help:"The path to the test manifests"`
	ProjectFile             string   `default:"upbound.yaml"                                                                                                                     help:"Path to project definition file."                                                            short:"f"`
	Repository              string   `help:"Repository for the built package. Overrides the repository specified in the project file."                                           optional:""`
	NoBuildCache            bool     `default:"false"                                                                                                                            help:"Don't cache image layers while building."`
	BuildCacheDir           string   `default:"~/.up/build-cache"                                                                                                                help:"Path to the build cache directory."                                                          type:"path"`
	MaxConcurrency          uint     `default:"8"                                                                                                                                env:"UP_MAX_CONCURRENCY"                                                                           help:"Maximum number of functions to build and push at once."`
	ControlPlaneGroup       string   `help:"The control plane group that the control plane to use is contained in. This defaults to the group specified in the current context."`
	ControlPlaneNamePrefix  string   `help:"Prefix of the control plane name to use. It will be created if not found."`
	ControlPlaneVersion     string   `help:"Version of Crossplane to use for the control plane. By default, the latest compatible version will be used."`
	Force                   bool     `alias:"allow-production"                                                                                                                   help:"Allow running on a non-development control plane."                                           name:"skip-control-plane-check"`
	Local                   bool     `help:"Use a local dev control plane, even if Spaces is available."`
	ClusterAdmin            bool     `default:"true"                                                                                                                             help:"Allow Crossplane cluster admin privileges in the local dev control plane. Defaults to true." negatable:""`
	LocalRegistryPath       string   `help:"Directory to use for local registry images. The default is system-dependent."`
	SkipControlPlaneCleanup bool     `help:"Skip cleanup of the control plane after the test run."                                                                               name:"skip-control-plane-cleanup"`
	UseCurrentContext       bool     `help:"Run the project with the current kubeconfig context rather than creating a new dev control plane."`
	CacheDir                string   `default:"~/.up/cache/"                                                                                                                     env:"CACHE_DIR"                                                                                    help:"Directory used for caching dependencies."               type:"path"`
	FunctionAnnotations     []string `help:"Override function annotations for all functions (compositionTests and operationTests). Can be repeated."                             placeholder:"KEY=VALUE"`

	Kubectl string `env:"KUBECTL" help:"Absolute path to the kubectl binary. Defaults to the one in $PATH." type:"path"`

	Public    bool `help:"Create new repositories with public visibility."`
	E2E       bool `help:"Run E2E tests"                                   name:"e2e"`
	Operation bool `help:"Run Operation tests"                             name:"operation"`

	projFS             afero.Fs
	testFS             afero.Fs
	functionIdentifier functions.Identifier
	schemaRunner       runner.SchemaRunner
	transport          http.RoundTripper
	m                  *project.DependencyManager
	r                  manager.ImageResolver
	keychain           authn.Keychain
	concurrency        uint
	proj               *project.WithVersion
}

//go:embed help/run.md
var runHelp string

func (c *runCmd) Help() string {
	return runHelp
}

// AfterApply processes flags and sets defaults.
func (c *runCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	c.concurrency = max(1, c.MaxConcurrency)

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

	// parse the project
	proj, err := project.ParseWithVersion(c.projFS, filepath.Base(c.ProjectFile))
	if err != nil {
		return err
	}
	proj.Default()

	c.proj = proj

	c.testFS = afero.NewBasePathFs(
		c.projFS, proj.Spec.Paths.Tests,
	)

	c.functionIdentifier = functions.DefaultIdentifier
	c.schemaRunner = runner.NewRealSchemaRunner(
		runner.WithImageConfig(proj.Spec.ImageConfig),
	)
	c.transport = http.DefaultTransport
	c.keychain = upCtx.RegistryKeychain()

	r := image.NewResolver(
		image.WithImageConfig(proj.Spec.ImageConfig),
		image.WithFetcher(
			image.NewLocalFetcher(
				image.WithKeychain(upCtx.RegistryKeychain()),
			),
		),
	)

	cchFS := afero.NewBasePathFs(afero.NewOsFs(), c.CacheDir)
	m, err := project.NewDependencyManager(upCtx, proj.Project, c.projFS,
		project.WithCacheFS(cchFS),
	)
	if err != nil {
		return err
	}
	c.m = m
	c.r = r

	if c.E2E {
		// set the default prefix
		if c.ControlPlaneNamePrefix == "" {
			c.ControlPlaneNamePrefix = fmt.Sprintf("%s-%s", proj.Name, "uptest")
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

	logger := logging.NewNopLogger()
	kongCtx.BindTo(logger, (*logging.Logger)(nil))

	return nil
}

// Run is the body of the command.
func (c *runCmd) Run(ctx context.Context, upCtx *upbound.Context, printer upterm.Printer, log logging.Logger) error {
	if c.UseCurrentContext && !c.Force {
		if err := c.confirmUseCurrentContext(upCtx); err != nil {
			return err
		}
	}

	var err error
	var parsedTests []any
	if err = printer.WrapWithSuccessSpinner(
		"Parsing tests",
		func() error {
			if err := apis.GenerateSchema(ctx, c.m.SchemaManager()); err != nil {
				return errors.Wrap(err, "unable to generate meta apis schemas")
			}

			testBuilder := test.NewBuilder(
				test.BuildWithSchemaRunner(c.schemaRunner),
			)
			parsedTests, err = testBuilder.Build(ctx, c.testFS, c.Patterns, c.proj.Spec.Paths.Tests)
			if err != nil {
				return errors.Wrap(err, "failed to generate test files")
			}

			return nil
		},
	); err != nil {
		return err
	}

	if len(parsedTests) == 0 {
		printer.PrintError("No test files found")
		return nil
	}

	var (
		ttotal   int
		tsuccess int
		terr     int
	)

	switch {
	case c.E2E:
		tests, err := e2etest.Convert(parsedTests)
		if err != nil {
			return errors.Wrap(err, "unable to validate e2e tests")
		}

		ttotal, tsuccess, terr, err = c.runE2ETests(ctx, upCtx, tests, printer)
		if err != nil {
			displayTestResults(printer, ttotal, tsuccess, terr)
			return errors.Wrap(err, "unable to execute e2e tests")
		}
	case c.Operation:
		tests, err := operationtest.Convert(parsedTests)
		if err != nil {
			return errors.Wrap(err, "unable to validate operation tests")
		}

		ttotal, tsuccess, terr, err = c.runOperationTests(ctx, upCtx, log, tests, printer)
		if err != nil {
			displayTestResults(printer, ttotal, tsuccess, terr)
			return errors.Wrap(err, "unable to execute operation tests")
		}
	default:
		tests, err := compositiontest.Convert(parsedTests)
		if err != nil {
			return errors.Wrap(err, "unable to validate composition tests")
		}
		ttotal, tsuccess, terr, err = c.runCompositionTests(ctx, upCtx, log, tests, printer)
		if err != nil {
			displayTestResults(printer, ttotal, tsuccess, terr)
			return errors.Wrap(err, "unable to execute composition tests")
		}
	}

	displayTestResults(printer, ttotal, tsuccess, terr)
	// Return an error if there were failed tests
	if terr > 0 {
		return err
	}

	return nil
}

const useCurrentContextConfirmFmt = `Running e2e tests on an existing control plane can be destructive.
Are you sure you want to use kubeconfig context %q?`

func (c *runCmd) confirmUseCurrentContext(upCtx *upbound.Context) error {
	ctxName, err := upCtx.GetCurrentContextName()
	if err != nil {
		return err
	}

	proceed, err := upterm.Confirm(fmt.Sprintf(useCurrentContextConfirmFmt, ctxName), false)
	if err != nil {
		return err
	}
	if !proceed {
		return errors.New("operation canceled")
	}

	return nil
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

func (c *runCmd) pushOrLoadPackages(ctx context.Context, upCtx *upbound.Context, imgMap project.ImageTagMap, devCtp ctp.DevControlPlane, printer upterm.Printer) (name.Tag, error) {
	if sl, ok := devCtp.(ctp.SideloadingControlPlane); ok {
		tagStr := fmt.Sprintf("%s:v0.0.0-%d", c.proj.Spec.Repository, time.Now().Unix())
		tag, err := name.NewTag(tagStr, name.StrictValidation)
		if err != nil {
			return tag, errors.Wrap(err, "failed to construct image tag")
		}

		err = printer.WrapWithSuccessSpinner("Loading packages into control plane", func() error {
			return sl.Sideload(ctx, imgMap, tag)
		})

		return tag, err
	}

	var generatedTag name.Tag
	err := printer.WrapAsyncWithSuccessSpinners(func(ch async.EventChannel) error {
		pusher := project.NewPusher(
			project.PushWithUpboundContext(upCtx),
			project.PushWithTransport(c.transport),
			project.PushWithAuthKeychain(c.keychain),
			project.PushWithMaxConcurrency(c.concurrency),
		)

		opts := []project.PushOption{
			project.PushWithEventChannel(ch),
			project.PushWithCreatePublicRepositories(c.Public),
		}

		var err error
		generatedTag, err = pusher.Push(ctx, c.proj.Project, imgMap, opts...)
		return err
	})

	return generatedTag, err
}

func displayTestResults(p upterm.Printer, ttotal, tsuccess, terr int) {
	printlnFunc := p.PrintSuccess
	if terr > 0 {
		printlnFunc = p.PrintError
	}

	printlnFunc()
	printlnFunc("Tests Summary:")
	printlnFunc("------------------")
	printlnFunc("Total Tests Executed:", ttotal)
	printlnFunc("Passed tests:        ", tsuccess)
	printlnFunc("Failed tests:        ", terr)
}

func assertions(ctx context.Context, output, testName string, expectedAssertions []runtime.RawExtension, ch async.EventChannel, p upterm.Printer) error {
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
		p.Print(formatDiffError(finalErr))
		ch.SendEvent(statusStage, async.EventStatusFailure)
		return finalErr
	}

	ch.SendEvent(statusStage, async.EventStatusSuccess)

	return nil
}

func formatDiffError(err error) string {
	var (
		red    = lipgloss.NewStyle().Foreground(style.RedColor)
		yellow = lipgloss.NewStyle().Foreground(style.YellowColor)
		green  = lipgloss.NewStyle().Foreground(style.GreenColor)
	)

	errorLines := strings.SplitSeq(err.Error(), "\n")

	bld := &strings.Builder{}
	for line := range errorLines {
		switch {
		case strings.HasPrefix(line, "---") && !strings.HasPrefix(line, "----"):
			bld.WriteString(red.Render(line)) // Expected
		case strings.HasPrefix(line, "+++"):
			bld.WriteString(green.Render(line)) // Actual
		case strings.HasPrefix(line, "@@"):
			bld.WriteString(yellow.Render(line)) // Context lines
		case strings.HasPrefix(line, "- "):
			bld.WriteString(red.Render(line)) // Removed lines
		case strings.HasPrefix(line, "+ "):
			bld.WriteString(green.Render(line)) // Added lines
		default:
			bld.WriteString(line) // Default text
		}
		bld.WriteString("\n")
	}

	return bld.String()
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
		var obj map[string]any
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
			// Strip ownerReferences from the rendered copy unless the expected
			// manifest explicitly asserts them. They are set by Crossplane on
			// composed resources and are noise in the diff when not under test.
			renderedForCheck := *rendered.DeepCopy()
			if len(expected.GetOwnerReferences()) == 0 {
				renderedForCheck.SetOwnerReferences(nil)
			}

			checkErrs, err := chainsawchecks.Check(ctx, chainsawapis.DefaultCompilers, renderedForCheck.UnstructuredContent(), chainsawapis.NewBindings(), ptr.To(chainsawv1alpha1.NewCheck(expected.UnstructuredContent())))
			if err != nil {
				return fmt.Errorf("error during manifest check: %w", err)
			}

			if len(checkErrs) == 0 {
				return nil
			}

			return chainsawerrors.ResourceError(
				chainsawcompilers.DefaultCompilers,
				expected,
				renderedForCheck,
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

// truncateAndValidateName ensures the final name is <=63 chars and valid as a DNS-1123 label.
func truncateAndValidateName(prefix, name string) (string, error) {
	fullName := fmt.Sprintf("%s-%s", prefix, name)

	// Truncate to 63 characters max
	if len(fullName) > 63 {
		fullName = fullName[:63]
	}

	// Trim trailing dashes in case truncation ends with '-'
	fullName = strings.TrimRight(fullName, "-")
	if errs := validation.IsDNS1123Label(fullName); len(errs) > 0 {
		return "", fmt.Errorf("invalid DNS-1123 label: %v", errs)
	}

	return fullName, nil
}
