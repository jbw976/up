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
	"syscall"
	"time"

	v1cache "github.com/google/go-containerregistry/pkg/v1/cache"
	"github.com/spf13/afero"
	"sigs.k8s.io/controller-runtime/pkg/scheme"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	v1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"
	xpkgv1beta1 "github.com/crossplane/crossplane/v2/apis/pkg/v1beta1"
	uptest "github.com/crossplane/uptest/pkg"

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
	"github.com/upbound/up/pkg/apis/project/v2alpha1"
)

func (c *runCmd) runE2ETests(ctx context.Context, upCtx *upbound.Context, tests []e2etest.E2ETest) (int, int, int, error) {
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
	)

	var imgMap project.ImageTagMap
	if err = c.asyncWrapper(func(ch async.EventChannel) error {
		imgMap, err = b.Build(ctx, upCtx, c.proj, c.projFS,
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

func (c *runCmd) executeE2ETest(ctx context.Context, upCtx *upbound.Context, proj *v2alpha1.Project, imgMap project.ImageTagMap, test e2etest.E2ETest) error { //nolint:gocognit // This could be refactored a bit, but isn't too bad.
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

			if test.Spec.Crossplane != nil {
				opts = append(opts, ctp.WithSpacesCrossplaneSpec(*test.Spec.Crossplane))
				if test.Spec.Crossplane.Version != nil {
					opts = append(opts, ctp.WithLocalCrossplaneVersion(*test.Spec.Crossplane.Version))
				}
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

	// Handle OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan) // Ensure we stop receiving signals after function exits

	// Channel to signal function return
	retChan := make(chan struct{})

	go func() {
		select {
		case <-sigChan:
			log.Println("Received termination signal")
			if !c.SkipControlPlaneCleanup {
				log.Println("Cleaning up control plane...")
				if err := devCtp.Teardown(ctx, c.Force); err != nil {
					log.Printf("error during control plane deletion %v", err)
				}
			}
			os.Exit(1)
		case <-retChan:
			return
		}
	}()

	defer func() {
		// Send signal to cleanup goroutine
		close(retChan)

		// Clean up the dev control plane. We do this here rather than in the
		// goroutine above because we can't guarantee it completes before `up`
		// exits, and we risk leaving the control plane behind.
		if !c.SkipControlPlaneCleanup {
			if err := devCtp.Teardown(ctx, c.Force); err != nil {
				log.Printf("error during control plane deletion %v", err)
			}
		}
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

		manifestFile := filepath.Join(tempDir, fmt.Sprintf("manifest-%d.yaml", i))
		if err := os.WriteFile(manifestFile, manifest.Raw, 0o600); err != nil {
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
