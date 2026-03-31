// Copyright 2025 Upbound Inc.
// All rights reserved

// Package pkg contains functions for handling install crossplane packages
package pkg

import (
	"context"
	"fmt"
	"time"

	"github.com/alecthomas/kong"
	"github.com/google/go-containerregistry/pkg/name"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/e2e-framework/klient/wait"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	xpv1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"

	upboundpkgv1beta1 "github.com/upbound/up-sdk-go/apis/pkg/v1beta1"
	"github.com/upbound/up/internal/resources"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/internal/xpkg"
	"github.com/upbound/up/internal/xpkg/dep"
	"github.com/upbound/up/internal/xpkg/dep/resolver/image"
)

const errUnknownPkgType = "provided package type is unknown"

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *installCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	switch kongCtx.Selected().Vars()["package_type"] {
	case xpv1.ProviderKind:
		c.gvr = xpv1.ProviderGroupVersionKind.GroupVersion().WithResource("providers")
		c.kind = xpv1.ProviderKind
	case xpv1.ConfigurationKind:
		c.gvr = xpv1.ConfigurationGroupVersionKind.GroupVersion().WithResource("configurations")
		c.kind = xpv1.ConfigurationKind
	case xpv1.FunctionKind:
		c.gvr = xpv1.FunctionGroupVersionKind.GroupVersion().WithResource("functions")
		c.kind = xpv1.FunctionKind
	case upboundpkgv1beta1.AddOnKind:
		c.gvr = upboundpkgv1beta1.AddOnGroupVersionKind.GroupVersion().WithResource("addons")
		c.kind = upboundpkgv1beta1.AddOnKind
	default:
		return errors.New(errUnknownPkgType)
	}

	c.i = image.NewResolver(
		image.WithFetcher(
			image.NewLocalFetcher(
				image.WithKeychain(upCtx.RegistryKeychain()),
			),
		),
	)

	return nil
}

// installCmd installs a package.
type installCmd struct {
	gvr  schema.GroupVersionResource
	kind string

	i *image.Resolver

	Package string `arg:"" help:"Reference to the ${package_type}."`

	// NOTE(hasheddan): kong automatically cleans paths tagged with existingfile.
	Name               string        `help:"Name of ${package_type}."`
	PackagePullSecrets []string      `help:"List of secrets used to pull ${package_type}."`
	Wait               time.Duration `help:"Wait duration for successful ${package_type} installation." short:"w"`
}

// Run executes the install command.
func (c *installCmd) Run(ctx context.Context, p upterm.Printer, upCtx *upbound.Context, client client.Client) error {
	// Resolve tag to handle latest cases
	d := dep.New(c.Package)
	tag, err := c.i.ResolveTag(ctx, d)
	if err != nil {
		return err
	}

	// Parse and resolve reference
	ref, err := name.ParseReference(c.Package, name.WithDefaultRegistry(upCtx.RegistryEndpoint.Hostname()))
	if err != nil {
		return err
	}

	var updatedRef name.Tag
	if tagRef, ok := ref.(name.Tag); ok {
		updatedRef, err = name.NewTag(fmt.Sprintf("%s:%s", tagRef.Repository, tag), name.StrictValidation)
		if err != nil {
			return err
		}
	} else {
		return errors.Errorf("unsupported reference type: %T", ref)
	}

	// Set default name if not provided
	if c.Name == "" {
		c.Name = xpkg.ToDNSLabel(ref.Context().RepositoryStr())
	}

	// Prepare package pull secrets
	packagePullSecrets := make([]corev1.LocalObjectReference, len(c.PackagePullSecrets))
	for i, s := range c.PackagePullSecrets {
		packagePullSecrets[i] = corev1.LocalObjectReference{Name: s}
	}

	// Create the resource
	resource := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": c.gvr.GroupVersion().String(),
			"kind":       c.kind,
			"metadata": map[string]any{
				"name": c.Name,
			},
			"spec": map[string]any{
				"package":            updatedRef.Name(),
				"packagePullSecrets": packagePullSecrets,
			},
		},
	}
	if err := client.Create(ctx, resource); err != nil {
		return err
	}

	// Return early if wait duration is not provided
	if c.Wait == 0 {
		p.Printfln("%s installed", c.Name)
		return nil
	}

	// Wait for the resource to become healthy
	p.Printfln("%s installed. Waiting to become healthy...", c.Name)
	waitFunc := func(ctx context.Context) (bool, error) {
		if err := client.Get(ctx, types.NamespacedName{Name: c.Name}, resource); err != nil {
			return false, err
		}
		// Convert resource to Package type to check conditions
		pkg := resources.Package{Unstructured: *resource}
		return pkg.GetInstalled() && pkg.GetHealthy(), nil
	}

	if err := wait.For(waitFunc, wait.WithImmediate(), wait.WithInterval(2*time.Second), wait.WithContext(ctx)); err != nil {
		return errors.Wrap(err, "error while waiting for package to become healthy")
	}

	p.Printfln("%s installed and healthy", c.Name)
	return nil
}
