// Copyright 2025 Upbound Inc.
// All rights reserved

package test

import (
	"context"
	"fmt"
	"time"

	"github.com/pterm/pterm"
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	v1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"

	"github.com/upbound/up/internal/async"
	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/internal/render"
	"github.com/upbound/up/internal/upbound"
	compositiontest "github.com/upbound/up/pkg/apis/compositiontest/v1alpha1"
)

func (c *runCmd) runCompositionTests(ctx context.Context, upCtx *upbound.Context, log logging.Logger, tests []compositiontest.CompositionTest) (int, int, int, error) {
	total, success, errs := 0, 0, 0

	var efns []v1.Function
	err := c.asyncWrapper(func(ch async.EventChannel) error {
		functionOptions := render.FunctionOptions{
			Project: c.proj.Project,
			// Use the original projFS here so schema generation knows the real
			// path.
			ProjFS:             c.projFS,
			Concurrency:        c.concurrency,
			NoBuildCache:       c.NoBuildCache,
			BuildCacheDir:      c.BuildCacheDir,
			DependencyManager:  c.m,
			FunctionIdentifier: c.functionIdentifier,
			EventChannel:       ch,
		}

		fns, err := render.BuildEmbeddedFunctionsLocalDaemon(ctx, upCtx, functionOptions)
		if err != nil {
			return err
		}
		efns = fns

		return nil
	})
	if err != nil {
		return 0, 0, 0, err
	}

	// Create an overlay filesystem so we can write resources to temporary files
	// that will be used during render only.
	overlayFS := filesystem.MemOverlay(c.projFS)
	var finalErr error
	for _, test := range tests {
		total++

		testFiles, err := c.prepareTestFiles(overlayFS, test)
		if err != nil {
			errs++
			finalErr = errors.Join(finalErr, err)
			continue
		}

		options := c.buildRenderOptions(overlayFS, test, testFiles)
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
			return assertions(ctx, output, test.Name, test.Spec.AssertResources, ch)
		}); err != nil {
			errs++
			finalErr = errors.Join(finalErr, err)
			continue
		}
		success++
	}

	return total, success, errs, finalErr
}

type testFilePaths struct {
	observedResources string
	extraResources    string
	xr                string
	composition       string
	xrd               string
	context           map[string]string
}

func (c *runCmd) prepareTestFiles(overlayFS afero.Fs, test compositiontest.CompositionTest) (*testFilePaths, error) {
	paths := &testFilePaths{
		context: make(map[string]string),
	}

	observedResourcesPath, err := writeToFile(overlayFS, test.Spec.ObservedResources, "observed")
	if err != nil {
		return nil, err
	}
	paths.observedResources = observedResourcesPath

	extraResourcesPath, err := writeToFile(overlayFS, test.Spec.ExtraResources, "extraresources")
	if err != nil {
		return nil, err
	}
	paths.extraResources = extraResourcesPath

	xrPath, err := c.resolveResourcePath(overlayFS, test.Spec.XRPath, test.Spec.XR, "xr")
	if err != nil {
		return nil, err
	}
	paths.xr = xrPath

	compositionPath, err := c.resolveResourcePath(overlayFS, test.Spec.CompositionPath, test.Spec.Composition, "composition")
	if err != nil {
		return nil, err
	}
	paths.composition = compositionPath

	xrdPath, err := c.resolveResourcePath(overlayFS, test.Spec.XRDPath, test.Spec.XRD, "xrd")
	if err != nil {
		return nil, err
	}
	paths.xrd = xrdPath

	for key, value := range test.Spec.Context {
		path, err := writeContextToFile(overlayFS, value, fmt.Sprintf("context-%s", key))
		if err != nil {
			return nil, err
		}
		paths.context[key] = path
	}

	return paths, nil
}

func (c *runCmd) resolveResourcePath(overlayFS afero.Fs, existingPath string, rawResource runtime.RawExtension, prefix string) (string, error) {
	if len(rawResource.Raw) > 0 {
		return writeToFile(overlayFS, []runtime.RawExtension{rawResource}, prefix)
	}
	return existingPath, nil
}

func (c *runCmd) buildRenderOptions(overlayFS afero.Fs, test compositiontest.CompositionTest, paths *testFilePaths) render.Options {
	return render.Options{
		Project:                c.proj.Project,
		ProjFS:                 overlayFS,
		IncludeFullXR:          true,
		IncludeFunctionResults: true,
		IncludeContext:         true,
		ObservedResources:      paths.observedResources,
		FunctionCredentials:    test.Spec.FunctionCredentialsPath,
		ExtraResources:         paths.extraResources,
		CompositeResource:      paths.xr,
		Composition:            paths.composition,
		XRD:                    paths.xrd,
		ContextFiles:           paths.context,
		Concurrency:            c.concurrency,
		ImageResolver:          c.r,
		FunctionAnnotations:    c.FunctionAnnotations,
	}
}
