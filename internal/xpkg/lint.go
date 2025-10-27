// Copyright 2025 Upbound Inc.
// All rights reserved

package xpkg

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	admv1 "k8s.io/api/admissionregistration/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	extv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	v1 "github.com/crossplane/crossplane/v2/apis/apiextensions/v1"
	extv1alpha1 "github.com/crossplane/crossplane/v2/apis/apiextensions/v1alpha1"
	v2 "github.com/crossplane/crossplane/v2/apis/apiextensions/v2"
	opsv1alpha1 "github.com/crossplane/crossplane/v2/apis/ops/v1alpha1"
	pkgmetav1 "github.com/crossplane/crossplane/v2/apis/pkg/meta/v1"

	upboundpkgmetav1alpha1 "github.com/upbound/up-sdk-go/apis/pkg/meta/v1alpha1"
	upboundpkgmetav1beta1 "github.com/upbound/up-sdk-go/apis/pkg/meta/v1beta1"
	"github.com/upbound/up/internal/xpkg/parser/linter"
	"github.com/upbound/up/internal/xpkg/scheme"
)

const (
	errNotExactlyOneMeta                 = "not exactly one package meta type"
	errNotMeta                           = "meta type is not a package"
	errNotMetaProvider                   = "package meta type is not Provider"
	errNotMetaConfiguration              = "package meta type is not Configuration"
	errNotMetaFunction                   = "package meta type is not Function"
	errNotMetaController                 = "package meta type is not Upbound Controller"
	errNotMetaAddOn                      = "package meta type is not Upbound AddOn"
	errNotCRD                            = "object is not a CRD"
	errNotMRD                            = "object is not an MRD"
	errNotMutatingWebhookConfiguration   = "object is not a MutatingWebhookConfiguration"
	errNotValidatingWebhookConfiguration = "object is not a ValidatingWebhookConfiguration"
	errNotXRD                            = "object is not a CompositeResourceDefinition (XRD); got Group: %s, Version: %s, Kind: %s"
	errNotComposition                    = "object is not a Composition; got Group: %s, Version: %s, Kind: %s"
	errNotOperation                      = "object is not an Operation; got Group: %s, Version: %s, Kind: %s"
	errBadConstraints                    = "package version constraints are poorly formatted"
	errNoCRDs                            = "package doesn't contain any CRDs"
	errNotActivationPolicy               = "object is not an ManagedResourceActivationPolicy"
)

// NewProviderLinter is a convenience function for creating a package linter for
// providers.
func NewProviderLinter() linter.Linter {
	return linter.NewPackageLinter(linter.PackageLinterFns(OneMeta), linter.ObjectLinterFns(IsProvider, PackageValidSemver),
		linter.ObjectLinterFns(linter.Or(
			IsCRD,
			IsMRD,
			IsValidatingWebhookConfiguration,
			IsMutatingWebhookConfiguration,
		)))
}

// NewConfigurationLinter is a convenience function for creating a package linter for
// configurations.
// Since we generate CRDs from XRDs for the cache,
// a Configuration Package retrieved from the cache may include CRDs.
func NewConfigurationLinter() linter.Linter {
	return linter.NewPackageLinter(linter.PackageLinterFns(OneMeta), linter.ObjectLinterFns(IsConfiguration, PackageValidSemver), linter.ObjectLinterFns(linter.Or(IsXRD, IsComposition, IsCRD, IsOperation, IsActivationPolicy)))
}

// NewFunctionLinter is a convenience function for creating a package linter for
// functions.
func NewFunctionLinter() linter.Linter {
	return linter.NewPackageLinter(linter.PackageLinterFns(OneMeta), linter.ObjectLinterFns(IsFunction), linter.ObjectLinterFns(IsCRD))
}

// NewControllerLinter is a convenience function for creating a package linter for
// Upbound controllers.
func NewControllerLinter() linter.Linter {
	return linter.NewPackageLinter(linter.PackageLinterFns(OneMeta, AtLeastOneCRD), linter.ObjectLinterFns(IsController), linter.ObjectLinterFns(IsCRD))
}

// NewAddOnLinter is a convenience function for creating a package linter for
// Upbound AddOns.
func NewAddOnLinter() linter.Linter {
	return linter.NewPackageLinter(linter.PackageLinterFns(OneMeta), linter.ObjectLinterFns(IsAddOn), linter.ObjectLinterFns(IsCRD))
}

// OneMeta checks that there is only one meta object in the package.
func OneMeta(pkg linter.Package) error {
	if len(pkg.GetMeta()) != 1 {
		return errors.New(errNotExactlyOneMeta)
	}
	return nil
}

// AtLeastOneCRD checks that there is at least one CRD in the package.
func AtLeastOneCRD(pkg linter.Package) error {
	for _, o := range pkg.GetObjects() {
		if _, ok := o.(*extv1.CustomResourceDefinition); ok {
			return nil
		}
	}
	return errors.New(errNoCRDs)
}

// IsProvider checks that an object is a Provider meta type.
func IsProvider(o runtime.Object) error {
	po, _ := scheme.TryConvert(o, &pkgmetav1.Provider{})
	if _, ok := po.(*pkgmetav1.Provider); !ok {
		return errors.New(errNotMetaProvider)
	}
	return nil
}

// IsConfiguration checks that an object is a Configuration meta type.
func IsConfiguration(o runtime.Object) error {
	po, _ := scheme.TryConvert(o, &pkgmetav1.Configuration{})
	if _, ok := po.(*pkgmetav1.Configuration); !ok {
		return errors.New(errNotMetaConfiguration)
	}
	return nil
}

// IsFunction checks that an object is a Function meta type.
func IsFunction(o runtime.Object) error {
	po, _ := scheme.TryConvert(o, &pkgmetav1.Function{})
	if _, ok := po.(*pkgmetav1.Function); !ok {
		return errors.New(errNotMetaFunction)
	}
	return nil
}

// IsController checks that an object is a Controller meta type.
func IsController(o runtime.Object) error {
	po, _ := scheme.TryConvert(o, &upboundpkgmetav1alpha1.Controller{})
	if _, ok := po.(*upboundpkgmetav1alpha1.Controller); !ok {
		return errors.New(errNotMetaController)
	}
	return nil
}

// IsAddOn checks that an object is an AddOn meta type.
func IsAddOn(o runtime.Object) error {
	po, _ := scheme.TryConvert(o, &upboundpkgmetav1beta1.AddOn{})
	if _, ok := po.(*upboundpkgmetav1beta1.AddOn); !ok {
		return errors.New(errNotMetaAddOn)
	}
	return nil
}

// PackageValidSemver checks that the package uses valid semver ranges.
func PackageValidSemver(o runtime.Object) error {
	p, ok := scheme.TryConvertToPkg(o, &pkgmetav1.Provider{}, &pkgmetav1.Configuration{})
	if !ok {
		return errors.New(errNotMeta)
	}

	if p.GetCrossplaneConstraints() == nil {
		return nil
	}
	if _, err := semver.NewConstraint(p.GetCrossplaneConstraints().Version); err != nil {
		return errors.Wrap(err, errBadConstraints)
	}
	return nil
}

// IsCRD checks that an object is a CustomResourceDefinition.
func IsCRD(o runtime.Object) error {
	switch o.(type) {
	case *extv1beta1.CustomResourceDefinition, *extv1.CustomResourceDefinition:
		return nil
	default:
		return errors.New(errNotCRD)
	}
}

// IsMutatingWebhookConfiguration checks that an object is a MutatingWebhookConfiguration.
func IsMutatingWebhookConfiguration(o runtime.Object) error {
	switch o.(type) {
	case *admv1.MutatingWebhookConfiguration:
		return nil
	default:
		return errors.New(errNotMutatingWebhookConfiguration)
	}
}

// IsValidatingWebhookConfiguration checks that an object is a MutatingWebhookConfiguration.
func IsValidatingWebhookConfiguration(o runtime.Object) error {
	switch o.(type) {
	case *admv1.ValidatingWebhookConfiguration:
		return nil
	default:
		return errors.New(errNotValidatingWebhookConfiguration)
	}
}

// IsXRD checks that an object is a CompositeResourceDefinition.
func IsXRD(o runtime.Object) error {
	switch o.(type) {
	case *v1.CompositeResourceDefinition, *v2.CompositeResourceDefinition:
		return nil
	default:
		return fmt.Errorf(errNotXRD, o.GetObjectKind().GroupVersionKind().Group, o.GetObjectKind().GroupVersionKind().Version, o.GetObjectKind().GroupVersionKind().Kind)
	}
}

// IsComposition checks that an object is a Composition.
func IsComposition(o runtime.Object) error {
	if _, ok := o.(*v1.Composition); !ok {
		return fmt.Errorf(errNotComposition, o.GetObjectKind().GroupVersionKind().Group, o.GetObjectKind().GroupVersionKind().Version, o.GetObjectKind().GroupVersionKind().Kind)
	}
	return nil
}

// IsOperation checks that an object is a kind of operation.
func IsOperation(o runtime.Object) error {
	switch o.(type) {
	case *opsv1alpha1.Operation, *opsv1alpha1.CronOperation, *opsv1alpha1.WatchOperation:
		return nil
	default:
		return fmt.Errorf(errNotOperation, o.GetObjectKind().GroupVersionKind().Group, o.GetObjectKind().GroupVersionKind().Version, o.GetObjectKind().GroupVersionKind().Kind)
	}
}

// IsMRD checks that an object is a ManagedResourceDefinition.
func IsMRD(o runtime.Object) error {
	switch o.(type) {
	case *extv1alpha1.ManagedResourceDefinition:
		return nil
	default:
		return errors.New(errNotMRD)
	}
}

// IsActivationPolicy checks that an object is an ManagedResourceActivationPolicy.
func IsActivationPolicy(o runtime.Object) error {
	if _, ok := o.(*extv1alpha1.ManagedResourceActivationPolicy); !ok {
		return errors.New(errNotActivationPolicy)
	}

	return nil
}
