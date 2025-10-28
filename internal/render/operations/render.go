// Copyright 2025 Upbound Inc.
// All rights reserved

// Package operations contains functions for operation rendering
package operations

import (
	"bytes"
	"context"
	"io"

	"github.com/spf13/afero"
	"google.golang.org/grpc/grpclog"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	pkgv1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"
	oprender "github.com/crossplane/crossplane/v2/cmd/crank/alpha/render/op"
	xprender "github.com/crossplane/crossplane/v2/cmd/crank/render"

	"github.com/upbound/up/internal/async"
	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/render"
	"github.com/upbound/up/internal/xpkg/dep/manager"
	"github.com/upbound/up/internal/xpkg/functions"
	projectv2alpha1 "github.com/upbound/up/pkg/apis/project/v2alpha1"
)

// Options defines the configuration for rendering.
type Options struct {
	Project *projectv2alpha1.Project
	ProjFS  afero.Fs

	IncludeFullOperation   bool
	IncludeFunctionResults bool
	IncludeContext         bool

	Operation           string
	FunctionCredentials string
	RequiredResources   string

	ContextFiles  map[string]string
	ContextValues map[string]string
	Concurrency   uint

	FunctionAnnotations []string

	ImageResolver manager.ImageResolver
}

// OperationOptions defines the configuration for building embedded functions.
type OperationOptions struct {
	Project *projectv2alpha1.Project
	ProjFS  afero.Fs

	Concurrency uint

	NoBuildCache       bool
	BuildCacheDir      string
	ImageResolver      manager.ImageResolver
	FunctionIdentifier functions.Identifier
	DependencyManager  *project.DependencyManager
	EventChannel       async.EventChannel
}

// Render executes the rendering logic and returns YAML output as a string.
func Render(ctx context.Context, log logging.Logger, embeddedFunctions []pkgv1.Function, opts Options) (string, error) {
	// Use our enhanced loader that supports CronOperation and WatchOperation
	op, err := loadOperationWithTemplateSupport(opts.ProjFS, opts.Operation)
	if err != nil {
		return "", errors.Wrapf(err, "cannot load operation from %q", opts.Operation)
	}

	// Load function credentials
	var fcreds []corev1.Secret
	if opts.FunctionCredentials != "" {
		fcreds, err = xprender.LoadCredentials(opts.ProjFS, opts.FunctionCredentials)
		if err != nil {
			return "", errors.Wrapf(err, "cannot load secrets from %q", opts.FunctionCredentials)
		}
	}

	var rrs []unstructured.Unstructured
	if opts.RequiredResources != "" {
		rrs, err = xprender.LoadRequiredResources(opts.ProjFS, opts.RequiredResources)
		if err != nil {
			return "", errors.Wrapf(err, "cannot load required resources from %q", opts.RequiredResources)
		}
	}

	// Load context values
	fctx := make(map[string][]byte)
	for k, filename := range opts.ContextFiles {
		v, err := afero.ReadFile(opts.ProjFS, filename)
		if err != nil {
			return "", errors.Wrapf(err, "cannot read context value for key %q", k)
		}
		fctx[k] = v
	}
	for k, v := range opts.ContextValues {
		fctx[k] = []byte(v)
	}

	// Load additional functions
	fns, err := render.LoadFunctions(ctx, opts.Project, opts.ImageResolver)
	if err != nil {
		return "", errors.Wrap(err, "cannot load functions from project")
	}
	fns = append(fns, embeddedFunctions...)

	// Apply global annotation overrides to each function
	if err := xprender.OverrideFunctionAnnotations(fns, opts.FunctionAnnotations); err != nil {
		return "", errors.Wrap(err, "cannot apply function annotation overrides")
	}

	// Turn off gRPC log messages.
	// When a function starts slowly, gRPC logs a warning that it can't handle the first request.
	// This warning goes away once the function is ready, so it's safe to discard the logs using io.Discard.
	grpclog.SetLoggerV2(grpclog.NewLoggerV2(io.Discard, io.Discard, io.Discard))

	// Render the operation
	out, err := oprender.Render(ctx, log, oprender.Inputs{
		Operation:           op,
		Functions:           fns,
		FunctionCredentials: fcreds,
		RequiredResources:   rrs,
		Context:             fctx,
	})
	if err != nil {
		return "", errors.Wrap(err, "cannot render operation")
	}

	// Serialize output to YAML
	s := json.NewSerializerWithOptions(json.DefaultMetaFactory, nil, nil, json.SerializerOptions{Yaml: true})
	var result string
	result += "---\n"

	// Only include spec when IncludeFullOperation flag is set
	if opts.IncludeFullOperation {
		_ = fieldpath.Pave(out.Operation.Object).SetValue("spec", *op.Spec.DeepCopy())
	}

	// Always output the Operation (with metadata and status, optionally with spec)
	var buffer bytes.Buffer
	if err := s.Encode(out.Operation, &buffer); err != nil {
		return "", errors.Wrap(err, "failed to encode operation resource to YAML")
	}
	result += buffer.String()

	// Output rendered resources
	for _, res := range out.Resources {
		result += "---\n"
		buffer.Reset()
		if err := s.Encode(&res, &buffer); err != nil {
			return "", errors.Wrap(err, "failed to encode composed resource to YAML")
		}
		result += buffer.String()
	}

	// Encode FunctionResults if needed
	if opts.IncludeFunctionResults {
		for _, res := range out.Results {
			result += "---\n"
			buffer.Reset()
			if err := s.Encode(&res, &buffer); err != nil {
				return "", errors.Wrap(err, "failed to encode function result to YAML")
			}
			result += buffer.String()
		}
	}

	// Encode Context if needed
	if opts.IncludeContext {
		result += "---\n"
		buffer.Reset()
		if err := s.Encode(out.Context, &buffer); err != nil {
			return "", errors.Wrap(err, "failed to encode context to YAML")
		}
		result += buffer.String()
	}

	return result, nil
}
