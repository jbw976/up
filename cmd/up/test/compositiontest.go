// Copyright 2025 Upbound Inc.
// All rights reserved

package test

import (
	"context"
	"time"

	"github.com/pterm/pterm"
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
			Project: c.proj,
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

		observedResourcesPath, err := writeToFile(overlayFS, test.Spec.ObservedResources, "observed")
		if err != nil {
			errs++
			finalErr = errors.Join(finalErr, err)
			continue
		}

		extraResourcesPath, err := writeToFile(overlayFS, test.Spec.ExtraResources, "extraresources")
		if err != nil {
			errs++
			finalErr = errors.Join(finalErr, err)
			continue
		}

		xrPath := test.Spec.XRPath
		if len(test.Spec.XR.Raw) > 0 {
			path, err := writeToFile(overlayFS, []runtime.RawExtension{test.Spec.XR}, "xr")
			if err != nil {
				errs++
				finalErr = errors.Join(finalErr, err)
				continue
			}
			xrPath = path
		}

		compositionPath := test.Spec.CompositionPath
		if len(test.Spec.Composition.Raw) > 0 {
			path, err := writeToFile(overlayFS, []runtime.RawExtension{test.Spec.Composition}, "composition")
			if err != nil {
				errs++
				finalErr = errors.Join(finalErr, err)
				continue
			}
			compositionPath = path
		}

		xrdPath := test.Spec.XRDPath
		if len(test.Spec.XRD.Raw) > 0 {
			path, err := writeToFile(overlayFS, []runtime.RawExtension{test.Spec.XRD}, "xrd")
			if err != nil {
				errs++
				finalErr = errors.Join(finalErr, err)
				continue
			}
			xrdPath = path
		}

		options := render.Options{
			Project:                c.proj,
			ProjFS:                 overlayFS,
			IncludeFullXR:          true,
			IncludeFunctionResults: true,
			IncludeContext:         true,
			ObservedResources:      observedResourcesPath,
			FunctionCredentials:    test.Spec.FunctionCredentialsPath,
			ExtraResources:         extraResourcesPath,
			CompositeResource:      xrPath,
			Composition:            compositionPath,
			XRD:                    xrdPath,
			Concurrency:            c.concurrency,
			ImageResolver:          c.r,
			FunctionAnnotations:    c.FunctionAnnotations,
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
