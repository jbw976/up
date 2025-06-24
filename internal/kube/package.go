// Copyright 2025 Upbound Inc.
// All rights reserved

package kube

import (
	"context"
	"encoding/json"
	"time"

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

	backoff := wait.Backoff{
		Duration: 500 * time.Millisecond,
		Factor:   2.0,
		Jitter:   0.1,
		Steps:    5,
	}

	err := wait.ExponentialBackoff(backoff, func() (bool, error) {
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
	if err := waitForPackagesReady(ctx, cl, tag); err != nil {
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

func waitForPackagesReady(ctx context.Context, cl client.Client, tag name.Tag) error {
	nn := types.NamespacedName{
		Name: "lock",
	}
	var lock xpkgv1beta1.Lock

	backoff := wait.Backoff{
		Duration: 500 * time.Millisecond, // Initial delay
		Factor:   2.0,                    // Multiplier for each retry
		Jitter:   0.1,                    // Randomize delay slightly
		Steps:    10,                     // Maximum number of retries
	}

	return wait.ExponentialBackoff(backoff, func() (bool, error) {
		if err := cl.Get(ctx, nn, &lock); err != nil {
			return false, nil //nolint:nilerr // Retry the operation
		}

		cfgPkg, cfgFound := lookupLockPackage(lock.Packages, tag.Repository.String(), tag.TagStr())
		if !cfgFound {
			return false, nil
		}

		healthy, err := packageIsHealthy(ctx, cl, cfgPkg)
		if err != nil {
			return false, err
		}
		if !healthy {
			return false, nil
		}

		healthy, err = allDepsHealthy(ctx, cl, lock, cfgPkg)
		if err != nil {
			return false, err
		}

		return healthy, nil
	})
}

func allDepsHealthy(ctx context.Context, cl client.Client, lock xpkgv1beta1.Lock, pkg xpkgv1beta1.LockPackage) (bool, error) {
	for _, dep := range pkg.Dependencies {
		depPkg, found := lookupLockPackage(lock.Packages, dep.Package, dep.Constraints)
		if !found {
			// Dep is not in lock yet - no need to look at the rest.
			break
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

func lookupLockPackage(pkgs []xpkgv1beta1.LockPackage, source, version string) (xpkgv1beta1.LockPackage, bool) {
	for _, pkg := range pkgs {
		if pkg.Source == source {
			if version == "" || pkg.Version == version {
				return pkg, true
			}
		}
	}
	return xpkgv1beta1.LockPackage{}, false
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

	return resource.IsConditionTrue(pkg.GetCondition(commonv1.TypeHealthy)), nil
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

		backoff := wait.Backoff{
			Duration: 5 * time.Second, // Initial delay
			Factor:   2.0,             // Multiplier for each retry
			Jitter:   0.1,             // Randomize delay slightly
			Steps:    20,              // Maximum number of retries
		}

		// RBAC is async, hence wrap in a retry loop
		if err := wait.ExponentialBackoff(backoff, func() (bool, error) {
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
