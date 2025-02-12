// Copyright 2021 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kube

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
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
	if err := cl.Patch(ctx, cfg, client.Apply, client.ForceOwnership, client.FieldOwner("up-cli")); err != nil {
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

func waitForPackagesReady(ctx context.Context, cl client.Client, tag name.Tag) error {
	nn := types.NamespacedName{
		Name: "lock",
	}
	var lock xpkgv1beta1.Lock
	for {
		time.Sleep(500 * time.Millisecond)
		err := cl.Get(ctx, nn, &lock)
		if err != nil {
			return err
		}

		cfgPkg, cfgFound := lookupLockPackage(lock.Packages, tag.Repository.String(), tag.TagStr())
		if !cfgFound {
			// Configuration not in lock yet.
			continue
		}
		healthy, err := packageIsHealthy(ctx, cl, cfgPkg)
		if err != nil {
			return err
		}
		if !healthy {
			// Configuration is not healthy yet.
			continue
		}

		healthy, err = allDepsHealthy(ctx, cl, lock, cfgPkg)
		if err != nil {
			return err
		}
		if healthy {
			break
		}
	}
	return nil
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
	switch lpkg.Type {
	case xpkgv1beta1.ConfigurationPackageType:
		pkg = &xpkgv1.ConfigurationRevision{}

	case xpkgv1beta1.ProviderPackageType:
		pkg = &xpkgv1.ProviderRevision{}

	case xpkgv1beta1.FunctionPackageType:
		pkg = &xpkgv1.FunctionRevision{}
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
				return false, nil //nolint:nilerr // Retry the operation
			}
			return true, nil // Success, stop retrying
		}); err != nil {
			return errors.Wrap(err, "failed to apply resource after retries")
		}
	}
	return nil
}
