// Copyright 2025 Upbound Inc.
// All rights reserved

package test

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	v1cache "github.com/google/go-containerregistry/pkg/v1/cache"
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/scheme"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	v1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"
	xpkgv1beta1 "github.com/crossplane/crossplane/v2/apis/pkg/v1beta1"
	uptest "github.com/crossplane/uptest/v2/pkg"

	upboundpkgv1alpha1 "github.com/upbound/up-sdk-go/apis/pkg/v1alpha1"
	upboundpkgv1beta1 "github.com/upbound/up-sdk-go/apis/pkg/v1beta1"
	"github.com/upbound/up/cmd/up/project/common"
	"github.com/upbound/up/internal/async"
	"github.com/upbound/up/internal/ctp"
	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/oci/cache"
	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
	e2etest "github.com/upbound/up/pkg/apis/e2etest/v1alpha1"
)

// e2eTestResourceAnnotation is the annotation we apply to all resources that
// get created as part of a test. This allows us to identify test resources for
// cleanup.
const e2eTestResourceAnnotation = "cli.upbound.io/e2etest"

func (c *runCmd) runE2ETests(ctx context.Context, upCtx *upbound.Context, tests []e2etest.E2ETest) (int, int, int, error) {
	var err error
	c.Repository, err = project.DetermineRepository(upCtx, c.proj.Project, c.Repository)
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
		if err := project.Move(ctx, c.proj.Project, c.projFS, c.Repository); err != nil {
			return 0, 0, 0, errors.Wrap(err, "failed to update project repository")
		}
	}

	b := project.NewBuilder(
		project.BuildWithMaxConcurrency(c.concurrency),
		project.BuildWithFunctionIdentifier(c.functionIdentifier),
	)

	var imgMap project.ImageTagMap
	if err = c.asyncWrapper(func(ch async.EventChannel) error {
		imgMap, err = b.Build(ctx, upCtx, c.proj.Project, c.projFS,
			project.BuildWithEventChannel(ch),
			project.BuildWithImageLabels(common.ImageLabels(c)),
			project.BuildWithDependencyManager(c.m),
			project.BuildWithProjectBasePath(basePath),
		)
		return err
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

	total, success, errs := 0, 0, 0
	var finalErr error

	for _, test := range tests {
		total++
		err = c.executeE2ETest(ctx, upCtx, c.proj, imgMap, test)
		if err != nil {
			errs++
			finalErr = errors.Join(finalErr, err)
			continue
		}
		success++
	}

	return total, success, errs, finalErr
}

func (c *runCmd) executeE2ETest(ctx context.Context, upCtx *upbound.Context, proj *project.WithVersion, imgMap project.ImageTagMap, test e2etest.E2ETest) error { //nolint:gocognit // This could be refactored a bit, but isn't too bad.
	controlPlaneName, err := truncateAndValidateName(c.ControlPlaneNamePrefix, test.Name)
	if err != nil {
		return errors.Wrap(err, "failed to create control plane")
	}
	var devCtp ctp.DevControlPlane
	if err := c.asyncWrapper(func(ch async.EventChannel) error {
		var err error
		if c.UseCurrentContext {
			devCtp, err = ctp.NewKubeconfigDevControlPlane(ctx, upCtx)
		} else {
			opts := []ctp.EnsureDevControlPlaneOption{
				ctp.WithEventChannel(ch),
				ctp.WithSpacesGroup(c.ControlPlaneGroup),
				ctp.WithControlPlaneName(controlPlaneName),
				ctp.SkipDevCheck(c.Force),
				ctp.ForceLocal(c.Local),
				ctp.WithLocalRegistryDirectory(c.LocalRegistryPath),
				ctp.WithClusterAdmin(c.ClusterAdmin),
			}

			switch {
			case c.ControlPlaneVersion != "":
				opts = append(opts,
					ctp.WithLocalCrossplaneVersion(c.ControlPlaneVersion),
					ctp.WithSpacesCrossplaneVersionConstraint(c.ControlPlaneVersion),
				)
			case test.Spec.Crossplane != nil:
				opts = append(opts, ctp.WithSpacesCrossplaneSpec(*test.Spec.Crossplane))
				if test.Spec.Crossplane.Version != nil {
					opts = append(opts, ctp.WithLocalCrossplaneVersion(*test.Spec.Crossplane.Version))
				}
			case proj.IsV1():
				opts = append(opts, ctp.WithSpacesCrossplaneVersionConstraint("^v1.18.0-up.0"))
			default:
				opts = append(opts, ctp.WithSpacesCrossplaneVersionConstraint("^v2.0.0-up.0"))
			}

			devCtp, err = ctp.EnsureDevControlPlane(ctx, upCtx, opts...)
		}
		return err
	}); err != nil {
		return errors.Wrap(err, "failed to create control plane")
	}

	generatedTag, err := c.pushOrLoadPackages(ctx, upCtx, imgMap, devCtp)
	if err != nil {
		return err
	}

	// We need to clean up before we return, even when we receive a
	// SIGINT. Specifically, we need to:
	//
	// 1. Try to delete any resources created in the control plane (and report
	//    any we failed to delete) to avoid leaving resources behind in cloud
	//    accounts.
	// 2. Cleanup the dev control plane, so we don't leave docker containers or
	//    Spaces MCPs sitting around.
	//
	// In the SIGINT case we also don't want to block forever. If the user sends
	// another SIGINT we should return right away even if we're not done
	// cleaning up.

	// Handle OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan) // Ensure we stop receiving signals after function exits

	// Channel to trigger cleanup on function return.
	retChan := make(chan struct{})
	// Channel to wait for cleanup completion.
	cleanupDone := make(chan struct{})

	go func() { //nolint:contextcheck // We intentionally use a separate context for cleanup.
		defer func() {
			close(cleanupDone)
		}()

		// Create a separate context for cleanup operations that can be cancelled
		// when receiving multiple signals.
		cleanupCtx, cancelCleanup := context.WithCancel(context.Background())
		defer cancelCleanup()

		select {
		case <-sigChan:
			c.textPrinter.Println("Received signal, cleaning up...")

			// Listen for further signals and cancel cleanup.
			go func() {
				select {
				case <-sigChan:
					cancelCleanup()
				case <-cleanupCtx.Done():
					return
				}
			}()

			c.e2eCleanup(cleanupCtx, devCtp, test)

		case <-retChan:
			c.e2eCleanup(cleanupCtx, devCtp, test)
		}
	}()

	defer func() {
		// Trigger cleanup.
		close(retChan)

		// Wait for cleanup/teardown to complete before returning, to ensure
		// cleanup happens before up exits.
		<-cleanupDone
	}()

	ctpSchemeBuilders := []*scheme.Builder{
		v1.SchemeBuilder,
		xpkgv1beta1.SchemeBuilder,
		upboundpkgv1alpha1.SchemeBuilder,
		upboundpkgv1beta1.SchemeBuilder,
	}
	for _, bld := range ctpSchemeBuilders {
		if err := bld.AddToScheme(devCtp.Client().Scheme()); err != nil {
			return err
		}
	}

	if err := upterm.WrapWithSuccessSpinner(
		"Applying Init Resources",
		upterm.CheckmarkSuccessSpinner,
		func() error {
			return kube.ApplyResources(ctx, devCtp.Client(), test.Spec.InitResources)
		},
		c.printer,
	); err != nil {
		return errors.Wrap(err, "failed to apply init resources")
	}

	err = c.asyncWrapper(func(ch async.EventChannel) error {
		return kube.InstallConfiguration(ctx, devCtp.Client(), proj.Name, generatedTag, ch)
	})
	if err != nil {
		return errors.Wrapf(err, "failed to install package")
	}

	if err := upterm.WrapWithSuccessSpinner(
		"Applying Extra Resources",
		upterm.CheckmarkSuccessSpinner,
		func() error {
			return kube.ApplyResources(ctx, devCtp.Client(), test.Spec.ExtraResources)
		},
		c.printer,
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

		// Parse the manifest to add annotations
		obj := &unstructured.Unstructured{}
		if err := obj.UnmarshalJSON(manifest.Raw); err != nil {
			return errors.Wrapf(err, "failed to unmarshal manifest %d", i)
		}

		// Add the uptest annotation
		annotations := obj.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[e2eTestResourceAnnotation] = "true"
		obj.SetAnnotations(annotations)

		// Marshal back to JSON
		annotatedManifest, err := obj.MarshalJSON()
		if err != nil {
			return errors.Wrapf(err, "failed to marshal manifest %d with annotations", i)
		}

		manifestFile := filepath.Join(tempDir, fmt.Sprintf("manifest-%d.yaml", i))
		if err := os.WriteFile(manifestFile, annotatedManifest, 0o600); err != nil {
			return errors.Wrapf(err, "failed writing manifest %d to file", i)
		}

		manifestPaths = append(manifestPaths, manifestFile)
	}

	kubeconfigPath, err := writeClientConfig(devCtp.Kubeconfig(), tempDir)
	if err != nil {
		return errors.Wrap(err, "error getting kubeconfig of controlplane")
	}

	vars := map[string]string{
		"KUBECTL":    c.Kubectl,
		"KUBECONFIG": kubeconfigPath,
	}

	cleanupEnvVars, err := setEnvVars(vars)
	if err != nil {
		return errors.Wrap(err, "failed setting environment variables")
	}
	defer cleanupEnvVars()

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
		SetUseLibraryMode(true).
		Build()

	if err := uptest.RunTest(automatedTest); err != nil {
		return errors.Wrap(err, "uptest failed")
	}

	return nil
}

// executeCleanup performs cleanup of test resources and returns the result.
func (c *runCmd) executeCleanup(ctx context.Context, devCtp ctp.DevControlPlane, test e2etest.E2ETest) (*ctp.CleanupResult, error) {
	var result *ctp.CleanupResult
	var cleanupErr error

	_ = c.asyncWrapper(func(ch async.EventChannel) error {
		// Use CleanupTimeoutSeconds from test spec, default to 600 seconds (10 minutes)
		cleanupTimeout := 600
		if test.Spec.CleanupTimeoutSeconds != nil {
			cleanupTimeout = *test.Spec.CleanupTimeoutSeconds
		}
		result, cleanupErr = devCtp.Cleanup(ctx,
			ctp.WithCleanupEventChannel(ch),
			ctp.WithCleanupTimeout(time.Duration(cleanupTimeout)*time.Second),
			ctp.WithCleanupAnnotation(e2eTestResourceAnnotation))
		return nil
	})

	return result, cleanupErr
}

// reportCleanupResult prints the cleanup result to the console.
func (c *runCmd) reportCleanupResult(result *ctp.CleanupResult, err error, detailed bool) {
	if err != nil {
		c.textPrinter.Printfln("Cleanup error: %v", err)
		return
	}

	if result == nil {
		return
	}

	if detailed {
		// Detailed output for interrupted cleanup
		c.textPrinter.Printfln("Cleanup summary: %d deleted, %d remaining after %d attempts",
			result.DeletedCount, result.RemainingCount, result.Attempts)

		if len(result.Resources) > 0 {
			c.textPrinter.Println("Resource cleanup details:")
			if err := c.printResources(result.Resources); err != nil {
				c.textPrinter.Printfln("Error printing resources: %v", err)
			}
		}
	} else {
		// Summary output for normal completion
		c.textPrinter.Printfln("Cleanup completed: %d deleted, %d remaining",
			result.DeletedCount, result.RemainingCount)

		// Only show the table if there are remaining resources
		remainingCount := 0
		for _, r := range result.Resources {
			if r.Status != "Deleted" {
				remainingCount++
			}
		}

		if remainingCount > 0 {
			c.textPrinter.Println("Warning: Some resources could not be removed:")
			if err := c.printResources(result.Resources); err != nil {
				c.textPrinter.Printfln("Error printing resources: %v", err)
			}
		}
	}
}

// e2eCleanup cleans up test resources and the dev control plane. It returns an
// exit code for the process, depending on how cleanup finishes.
func (c *runCmd) e2eCleanup(ctx context.Context, devCtp ctp.DevControlPlane, test e2etest.E2ETest) {
	if c.SkipControlPlaneCleanup {
		c.textPrinter.Println("Skipping cleanup due to --skip-control-plane-cleanup flag")
		return
	}

	c.textPrinter.Println("Cleaning up test resources...")
	result, cleanupErr := c.executeCleanup(ctx, devCtp, test)
	c.reportCleanupResult(result, cleanupErr, true)

	c.textPrinter.Println("Tearing down test control plane...")
	if err := devCtp.Teardown(ctx, c.Force); err != nil {
		c.textPrinter.Printfln("Error during control plane deletion: %v", err)
	}
	c.textPrinter.Println("Test control plane deleted")
}

func extractResourceFields(obj any) []string {
	r := obj.(ctp.GenericResource) //nolint:forcetypeassert // its always GenericResource

	name := fmt.Sprintf("%s.%s/%s",
		strings.ToLower(r.GVK.Kind),
		r.GVK.Group,
		r.Name)

	// Display external name or "-" if not present
	externalName := r.ExternalName
	if externalName == "" {
		externalName = "-"
	}

	// Truncate long messages for better readability
	displayMessage := r.Message
	if len(displayMessage) > 80 {
		displayMessage = displayMessage[:77] + "..."
	}

	return []string{name, externalName, r.Status, displayMessage}
}

func (c *runCmd) printResources(resources []ctp.GenericResource) error {
	if len(resources) == 0 {
		return nil
	}

	// Convert to []any for the printer
	items := make([]any, len(resources))
	for i, r := range resources {
		items[i] = r
	}

	resourceFieldNames := []string{"NAME", "EXTERNAL-NAME", "STATUS", "MESSAGE"}
	return c.printer.Print(items, resourceFieldNames, extractResourceFields)
}
