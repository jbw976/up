// Copyright 2025 Upbound Inc.
// All rights reserved

package kube

import (
	"context"
	"encoding/json"
	"math"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-containerregistry/pkg/name"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	commonv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	xpkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	xpkgv1beta1 "github.com/crossplane/crossplane/apis/pkg/v1beta1"

	"github.com/upbound/up/internal/async"
)

// InstallConfiguration will install crossplane packages to target controlplane.
func InstallConfiguration(ctx context.Context, cl client.Client, name string, tag name.Tag, ch async.EventChannel) error {
	pkgSource := tag.String()
	cfg := &xpkgv1.Configuration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: xpkgv1.SchemeGroupVersion.String(),
			Kind:       xpkgv1.ConfigurationKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: xpkgv1.ConfigurationSpec{
			PackageSpec: xpkgv1.PackageSpec{
				Package: pkgSource,
			},
		},
	}

	stage := "Installing package on development control plane"
	ch.SendEvent(stage, async.EventStatusStarted)

	err := retryWithBackoff(ctx, 2*time.Second, func(ctx context.Context) (bool, error) {
		err := cl.Patch(ctx, cfg, client.Apply, client.ForceOwnership, client.FieldOwner("up-cli"))
		if err != nil {
			if isRetryableServerError(err) {
				return false, nil // Retry
			}
			return false, err // Non-retryable
		}
		return true, nil
	})
	if err != nil {
		ch.SendEvent(stage, async.EventStatusFailure)
		return err
	}
	ch.SendEvent(stage, async.EventStatusSuccess)

	stage = "Waiting for package to be ready"
	ch.SendEvent(stage, async.EventStatusStarted)
	if err := waitForPackagesReady(ctx, cl, cfg); err != nil {
		ch.SendEvent(stage, async.EventStatusFailure)
		return err
	}
	ch.SendEvent(stage, async.EventStatusSuccess)

	return nil
}

func isRetryableServerError(err error) bool {
	if apierrors.IsTimeout(err) ||
		apierrors.IsInternalError(err) ||
		apierrors.IsServerTimeout(err) {
		return true
	}

	var statusErr *apierrors.StatusError
	if errors.As(err, &statusErr) {
		reason := statusErr.ErrStatus.Reason
		if reason == metav1.StatusReasonServiceUnavailable {
			return true
		}
	}

	return false
}

func waitForPackagesReady(ctx context.Context, cl client.Client, cfg *xpkgv1.Configuration) error {
	nn := types.NamespacedName{
		Name: "lock",
	}
	var lock xpkgv1beta1.Lock

	return retryWithBackoff(ctx, 5*time.Second, func(ctx context.Context) (bool, error) {
		// First, make sure the current revision reflects the latest version of
		// the package. If not, wait for a new revision to be created.
		cfgRev, revFound, err := getCurrentRevision(ctx, cl, cfg)
		if err != nil {
			return false, err
		}
		if !revFound {
			return false, nil
		}

		if cfgRev.GetSource() != cfg.GetSource() {
			// Revision is not current - wait for a new one.
			return false, nil
		}

		// Now we have the right revision. Wait for it to be healthy.
		if !packageHasHealthyConditions(cfgRev) {
			return false, nil
		}

		// Finally, find the package in the lock and make sure all its deps are
		// healthy.
		if err := cl.Get(ctx, nn, &lock); err != nil {
			if apierrors.IsNotFound(err) {
				// Lock not created yet - retry.
				return false, nil
			}
			return false, errors.Wrap(err, "failed to get lock")
		}

		var cfgPkg *xpkgv1beta1.LockPackage
		for _, pkg := range lock.Packages {
			if pkg.Name == cfgRev.Name {
				cfgPkg = &pkg
				break
			}
		}
		if cfgPkg == nil {
			// Package is not in the lock yet.
			return false, nil
		}

		healthy, err := allDepsHealthy(ctx, cl, lock, *cfgPkg)
		if err != nil {
			return false, err
		}

		return healthy, nil
	})
}

func getCurrentRevision(ctx context.Context, cl client.Client, cfg *xpkgv1.Configuration) (*xpkgv1.ConfigurationRevision, bool, error) {
	cfgNN := types.NamespacedName{
		Name: cfg.Name,
	}
	if err := cl.Get(ctx, cfgNN, cfg); err != nil {
		// Should exist since we created it before calling this
		// function. Don't retry for not found.
		return nil, false, errors.Wrap(err, "failed to get configuration")
	}

	if cfg.Status.CurrentRevision == "" {
		// Revision not created yet, caller should retry.
		return nil, false, nil
	}

	revNN := types.NamespacedName{
		Name: cfg.Status.CurrentRevision,
	}
	var cfgRev xpkgv1.ConfigurationRevision
	if err := cl.Get(ctx, revNN, &cfgRev); err != nil {
		if apierrors.IsNotFound(err) {
			// Revision not yet created, caller should retry.
			return nil, false, nil
		}
		return nil, false, errors.Wrap(err, "failed to get configuration revision")
	}

	return &cfgRev, true, nil
}

func allDepsHealthy(ctx context.Context, cl client.Client, lock xpkgv1beta1.Lock, pkg xpkgv1beta1.LockPackage) (bool, error) {
	for _, dep := range pkg.Dependencies {
		depPkg, found := lookupLockPackage(lock.Packages, dep.Package, dep.Constraints)
		if !found {
			// Dep is not in lock yet - no need to look at the rest.
			return false, nil
		}
		healthy, err := packageIsHealthy(ctx, cl, depPkg)
		if err != nil {
			return false, err
		}
		if !healthy {
			return false, nil
		}
	}

	return true, nil
}

// lookupLockPackage finds a package in the lock with the given source that
// satisfies the given version constraint. If the constraint does not parse as a
// semver constraint (e.g., if it's a digest), we look for an exactly matching
// version string.
func lookupLockPackage(pkgs []xpkgv1beta1.LockPackage, source, constraint string) (xpkgv1beta1.LockPackage, bool) {
	for _, pkg := range pkgs {
		if !sourcesEqual(pkg.Source, source) {
			continue
		}

		vc, err := semver.NewConstraint(constraint)
		if err != nil {
			// Not a semver, use exact matching.
			if pkg.Version == constraint {
				return pkg, true
			}
		}
		pv, err := semver.NewVersion(pkg.Version)
		if err != nil {
			// Not a semver, can't compare.
			continue
		}
		if vc.Check(pv) {
			return pkg, true
		}
	}
	return xpkgv1beta1.LockPackage{}, false
}

// sourcesEqual compares two package sources and returns true if they are equal,
// taking into account the silly special-casing that rewrites docker.io to
// index.docker.io and any other unexpected behavior that applies to image
// references. It will always return false if either a or b is an invalid OCI
// repository.
func sourcesEqual(a, b string) bool {
	ra, err := name.NewRepository(a, name.StrictValidation)
	if err != nil {
		return false
	}
	rb, err := name.NewRepository(b, name.StrictValidation)
	if err != nil {
		return false
	}

	return ra.String() == rb.String()
}

func packageIsHealthy(ctx context.Context, cl client.Client, lpkg xpkgv1beta1.LockPackage) (bool, error) {
	var pkg xpkgv1.PackageRevision

	if lpkg.Kind != nil {
		switch *lpkg.Kind {
		case xpkgv1.ConfigurationKind:
			pkg = &xpkgv1.ConfigurationRevision{}
		case xpkgv1.ProviderKind:
			pkg = &xpkgv1.ProviderRevision{}
		case xpkgv1.FunctionKind:
			pkg = &xpkgv1.FunctionRevision{}
		}
	}

	if lpkg.Type != nil {
		switch *lpkg.Type {
		case xpkgv1beta1.ConfigurationPackageType:
			pkg = &xpkgv1.ConfigurationRevision{}
		case xpkgv1beta1.ProviderPackageType:
			pkg = &xpkgv1.ProviderRevision{}
		case xpkgv1beta1.FunctionPackageType:
			pkg = &xpkgv1.FunctionRevision{}
		}
	}

	err := cl.Get(ctx, types.NamespacedName{Name: lpkg.Name}, pkg)
	if err != nil {
		return false, err
	}

	return packageHasHealthyConditions(pkg), nil
}

func packageHasHealthyConditions(pkg xpkgv1.PackageRevision) bool {
	// Crossplane v1.x sets the `Healthy` condition.
	v1Healthy := resource.IsConditionTrue(pkg.GetCondition(commonv1.TypeHealthy))
	// Crossplane v2.x sets the `RevisionHealthy`.
	v2Healthy := resource.IsConditionTrue(pkg.GetCondition(xpkgv1.TypeRevisionHealthy))

	// Crossplane v2 sets an additional `RuntimeHealthy` condition on packages
	// with a runtime (providers and functions).
	if _, ok := pkg.(xpkgv1.PackageRevisionWithRuntime); ok {
		v2Healthy = v2Healthy && resource.IsConditionTrue(pkg.GetCondition(xpkgv1.TypeRuntimeHealthy))
	}

	// Allow for either v1.x health or v2.x health, so we work correctly with
	// either version.
	return v1Healthy || v2Healthy
}

// ApplyResources installs arbitrary resources to the target control plane.
func ApplyResources(ctx context.Context, cl client.Client, resources []runtime.RawExtension) error {
	for _, raw := range resources {
		if len(raw.Raw) == 0 {
			return errors.New("encountered an invalid or empty raw resource")
		}

		obj := &unstructured.Unstructured{}
		if err := json.Unmarshal(raw.Raw, obj); err != nil {
			return errors.Wrap(err, "failed to unmarshal resource")
		}

		// RBAC is async, hence wrap in a retry loop
		if err := retryWithBackoff(ctx, 2*time.Second, func(ctx context.Context) (bool, error) {
			err := cl.Patch(ctx, obj, client.Apply, client.ForceOwnership, client.FieldOwner("up-cli"))
			if err != nil {
				// Check if this is a permanent error that shouldn't be retried
				if isPermanentError(err) {
					// Return the error to stop retrying
					return false, err
				}
				// For transient errors, return nil to continue retrying
				return false, nil
			}
			return true, nil // Success, stop retrying
		}); err != nil {
			return errors.Wrapf(err, "failed to apply resource %s/%s",
				obj.GetKind(), obj.GetName())
		}
	}
	return nil
}

// isPermanentError determines if an error should not be retried.
func isPermanentError(err error) bool {
	// Check for specific error types that indicate permanent failures
	if apierrors.IsBadRequest(err) ||
		apierrors.IsInvalid(err) ||
		apierrors.IsMethodNotSupported(err) ||
		apierrors.IsNotAcceptable(err) ||
		apierrors.IsUnsupportedMediaType(err) ||
		apierrors.IsUnauthorized(err) ||
		apierrors.IsForbidden(err) ||
		apierrors.IsRequestEntityTooLargeError(err) {
		return true
	}

	return false
}

// retryWithBackoff retries calls to fn with exponential backoff and jitter
// until fn returns true or the context is cancelled. maxWait is the maximum
// duration between calls.
//
// We use this instead of wait.ExponentialBackoffWithContext because we don't
// want to return early if the max wait is hit before the context is cancelled.
func retryWithBackoff(ctx context.Context, maxWait time.Duration, fn func(ctx context.Context) (bool, error)) error {
	backoff := wait.Backoff{
		Duration: 500 * time.Millisecond,
		Factor:   2.0,
		Jitter:   0.1,
		Cap:      maxWait,

		// We use Cap to control the max wait, rather than Steps, but need to
		// make sure Steps doesn't go to zero before Cap is reached.
		Steps: math.MaxInt32,
	}

	for {
		done, err := fn(ctx)
		if err != nil {
			return err
		}
		if done {
			return nil
		}

		sleep := backoff.Step()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sleep):
			// next loop!
		}
	}
}
