// Copyright 2025 Upbound Inc.
// All rights reserved

package test

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	v1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"

	"github.com/upbound/up/internal/async"
	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/internal/render"
	"github.com/upbound/up/internal/render/operations"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
	operationtest "github.com/upbound/up/pkg/apis/operationtest/v1alpha1"
)

func (c *runCmd) runOperationTests(ctx context.Context, upCtx *upbound.Context, log logging.Logger, tests []operationtest.OperationTest, printer upterm.Printer) (int, int, int, error) {
	total, success, errs := 0, 0, 0

	// Build embedded functions once for all tests
	var efns []v1.Function
	err := printer.WrapAsyncWithSuccessSpinners(func(ch async.EventChannel) error {
		functionOptions := render.FunctionOptions{
			Project:            c.proj.Project,
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

		testFiles, err := c.prepareOperationTestFiles(overlayFS, test)
		if err != nil {
			errs++
			finalErr = errors.Join(finalErr, err)
			continue
		}

		options := c.buildOperationRenderOptions(overlayFS, test, testFiles)

		// Set timeout context
		renderCtx, cancel := context.WithTimeout(ctx, time.Duration(test.Spec.TimeoutSeconds)*time.Second)
		defer cancel()

		// Render the operation
		output, err := operations.Render(renderCtx, log, efns, options)
		if err != nil {
			errs++
			finalErr = errors.Join(finalErr, errors.Wrapf(err, "failed to render operation for test %s", test.Name))
			printer.PrintError(err)
			continue
		}

		// Run assertions on the output
		if err = printer.WrapAsyncWithSuccessSpinners(func(ch async.EventChannel) error {
			return assertions(ctx, output, test.Name, test.Spec.AssertResources, ch, printer)
		}); err != nil {
			errs++
			finalErr = errors.Join(finalErr, err)
			continue
		}
		success++
	}

	return total, success, errs, finalErr
}

type operationTestFilePaths struct {
	operation         string
	requiredResources string
	context           map[string]string
}

func (c *runCmd) prepareOperationTestFiles(overlayFS afero.Fs, test operationtest.OperationTest) (*operationTestFilePaths, error) {
	paths := &operationTestFilePaths{
		context: make(map[string]string),
	}

	operationPath, err := c.resolveResourcePath(overlayFS, test.Spec.OperationPath, test.Spec.Operation, "operation")
	if err != nil {
		return nil, err
	}
	paths.operation = operationPath

	requiredResourcesPath, err := c.resolveResourcesPath(overlayFS, test.Spec.RequiredResourcesPath, test.Spec.RequiredResources)
	if err != nil {
		return nil, err
	}
	paths.requiredResources = requiredResourcesPath

	for key, value := range test.Spec.Context {
		path, err := writeContextToFile(overlayFS, value, fmt.Sprintf("context-%s", key))
		if err != nil {
			return nil, err
		}
		paths.context[key] = path
	}

	return paths, nil
}

func (c *runCmd) resolveResourcesPath(overlayFS afero.Fs, existingPath string, rawResources []runtime.RawExtension) (string, error) {
	if len(rawResources) > 0 {
		return writeToFile(overlayFS, rawResources, "requiredresources")
	}
	return existingPath, nil
}

func (c *runCmd) buildOperationRenderOptions(overlayFS afero.Fs, test operationtest.OperationTest, paths *operationTestFilePaths) operations.Options {
	return operations.Options{
		Project:                c.proj.Project,
		ProjFS:                 overlayFS,
		IncludeFullOperation:   true,
		IncludeFunctionResults: true,
		IncludeContext:         true,
		Operation:              paths.operation,
		FunctionCredentials:    test.Spec.FunctionCredentialsPath,
		RequiredResources:      paths.requiredResources,
		ContextFiles:           paths.context,
		Concurrency:            c.concurrency,
		ImageResolver:          c.r,
		FunctionAnnotations:    c.FunctionAnnotations,
	}
}
