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
	"github.com/upbound/up/internal/render/operations"
	"github.com/upbound/up/internal/upbound"
	operationtest "github.com/upbound/up/pkg/apis/operationtest/v1alpha1"
)

func (c *runCmd) runOperationTests(ctx context.Context, upCtx *upbound.Context, log logging.Logger, tests []operationtest.OperationTest) (int, int, int, error) {
	total, success, errs := 0, 0, 0

	// Build embedded functions once for all tests
	var efns []v1.Function
	err := c.asyncWrapper(func(ch async.EventChannel) error {
		functionOptions := render.FunctionOptions{
			Project:            c.proj,
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
			return errors.Wrap(err, "unable to build embedded functions")
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

		operationPath := test.Spec.OperationPath
		if len(test.Spec.Operation.Raw) > 0 {
			path, err := writeToFile(overlayFS, []runtime.RawExtension{test.Spec.Operation}, "operation")
			if err != nil {
				errs++
				finalErr = errors.Join(finalErr, err)
				continue
			}
			operationPath = path
		}

		requiredResourcesPath, err := writeToFile(overlayFS, test.Spec.RequiredResources, "requiredresources")
		if err != nil {
			errs++
			finalErr = errors.Join(finalErr, err)
			continue
		}

		// Create render options for the operation
		options := operations.Options{
			Project:                c.proj,
			ProjFS:                 overlayFS,
			IncludeFullOperation:   true,
			IncludeFunctionResults: true,
			IncludeContext:         true,
			Operation:              operationPath,
			FunctionCredentials:    test.Spec.FunctionCredentialsPath,
			RequiredResources:      requiredResourcesPath,
			Concurrency:            c.concurrency,
			ImageResolver:          c.r,
		}

		// Set timeout context
		renderCtx, cancel := context.WithTimeout(ctx, time.Duration(test.Spec.TimeoutSeconds)*time.Second)
		defer cancel()

		// Render the operation
		output, err := operations.Render(renderCtx, log, efns, options)
		if err != nil {
			errs++
			finalErr = errors.Join(finalErr, errors.Wrapf(err, "failed to render operation for test %s", test.Name))
			pterm.PrintOnError(err)
			continue
		}

		// Run assertions on the output
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
