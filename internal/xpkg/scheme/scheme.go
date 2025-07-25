// Copyright 2025 Upbound Inc.
// All rights reserved

// Package scheme provides utilities for working with Crossplane package
// metadata and objects.
package scheme

import (
	admv1 "k8s.io/api/admissionregistration/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	extv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	pkgmetav1alpha1 "github.com/crossplane/crossplane/apis/pkg/meta/v1alpha1"
	pkgmetav1beta1 "github.com/crossplane/crossplane/apis/pkg/meta/v1beta1"

	upboundpkgmetav1alpha1 "github.com/upbound/up-sdk-go/apis/pkg/meta/v1alpha1"
	projectv1alpha1 "github.com/upbound/up/pkg/apis/project/v1alpha1"
)

// BuildMetaScheme builds the default scheme used for identifying metadata in a
// Crossplane package.
func BuildMetaScheme() (*runtime.Scheme, error) {
	metaScheme := runtime.NewScheme()
	if err := pkgmetav1alpha1.AddToScheme(metaScheme); err != nil {
		return nil, err
	}
	if err := pkgmetav1beta1.AddToScheme(metaScheme); err != nil {
		return nil, err
	}
	if err := pkgmetav1.AddToScheme(metaScheme); err != nil {
		return nil, err
	}
	if err := projectv1alpha1.AddToScheme(metaScheme); err != nil {
		return nil, err
	}
	if err := upboundpkgmetav1alpha1.AddToScheme(metaScheme); err != nil {
		return nil, err
	}
	return metaScheme, nil
}

// BuildObjectScheme builds the default scheme used for identifying objects in a
// Crossplane package.
func BuildObjectScheme() (*runtime.Scheme, error) {
	objScheme := runtime.NewScheme()
	if err := v1.AddToScheme(objScheme); err != nil {
		return nil, err
	}
	if err := extv1beta1.AddToScheme(objScheme); err != nil {
		return nil, err
	}
	if err := extv1.AddToScheme(objScheme); err != nil {
		return nil, err
	}
	if err := admv1.AddToScheme(objScheme); err != nil {
		return nil, err
	}
	return objScheme, nil
}

// TryConvert converts the supplied object to the first supplied candidate that
// does not return an error. Returns the converted object and true when
// conversion succeeds, or the original object and false if it does not.
func TryConvert(obj runtime.Object, candidates ...conversion.Hub) (runtime.Object, bool) {
	// Note that if we already converted the supplied object to one of the
	// supplied Hubs in a previous call this will ensure we skip conversion if
	// and when it's called again.
	cvt, ok := obj.(conversion.Convertible)
	if !ok {
		return obj, false
	}

	for _, c := range candidates {
		if err := cvt.ConvertTo(c); err == nil {
			return c, true
		}
	}

	return obj, false
}

// TryConvertToPkg converts the supplied object to a pkgmeta.Pkg, if possible.
func TryConvertToPkg(obj runtime.Object, candidates ...conversion.Hub) (pkgmetav1.Pkg, bool) {
	po, _ := TryConvert(obj, candidates...)
	m, ok := po.(pkgmetav1.Pkg)
	return m, ok
}
