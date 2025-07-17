// Copyright 2025 Upbound Inc.
// All rights reserved

// Package license handles Upbound licenses.
package license

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/controller-manager/apis/licensing/v1alpha1"
)

const (
	errAddToScheme            = "failed to add license types to scheme"
	errFmtGetLicense          = "failed to get license %q"
	errFmtGetLicenseSecret    = "failed to get license secret %q"
	errFmtGetLicenseSecretKey = "license secret %q is missing key: %s"
)

var (
	// ErrCommunity is returned when attempting to get a license file from a
	// community license.
	ErrCommunity = errors.New("community license")
	// ErrLicenseNotFound is returned when a license resource with the default
	// name is not found.
	ErrLicenseNotFound = errors.New("license not found")
)

// FromUXPv2 returns a *v1alpha1.License from a controller-runtime client for a
// UXPv2 control plane.
func FromUXPv2(ctx context.Context, cl client.Client) (*v1alpha1.License, error) {
	if err := v1alpha1.AddToScheme(cl.Scheme()); err != nil {
		return nil, errors.Wrap(err, errAddToScheme)
	}

	var l v1alpha1.License
	if err := cl.Get(ctx, types.NamespacedName{Name: v1alpha1.LicenseName}, &l); err != nil {
		if kerrors.IsNotFound(err) {
			return nil, ErrLicenseNotFound
		}
		return nil, errors.Wrap(err, fmt.Sprintf(errFmtGetLicense, v1alpha1.LicenseName))
	}

	return &l, nil
}

// BytesFromUXPv2 returns a license file from a controller-runtime client for a
// UXPv2 control plane.
func BytesFromUXPv2(ctx context.Context, cl client.Client) ([]byte, error) {
	l, err := FromUXPv2(ctx, cl)
	if err != nil {
		return nil, err
	}

	if l.Spec.SecretRef == nil {
		return nil, ErrCommunity
	}

	var s corev1.Secret
	sn := types.NamespacedName{Name: l.Spec.SecretRef.Name, Namespace: l.Spec.SecretRef.Namespace}
	if err := cl.Get(ctx, sn, &s); err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf(errFmtGetLicenseSecret, sn.String()))
	}

	f, ok := s.Data[l.Spec.SecretRef.Key]
	if !ok {
		return nil, fmt.Errorf(errFmtGetLicenseSecretKey, sn.String(), l.Spec.SecretRef.Key)
	}

	return f, nil
}
