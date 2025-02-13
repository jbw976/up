// Copyright 2025 Upbound Inc.
// All rights reserved

package crd

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

// GetCRDVersion iterates over the versions defined in the CustomResourceDefinition (CRD).
// It returns the name of the version that has both "served" and "storage" fields set to true.
func GetCRDVersion(crd apiextensionsv1.CustomResourceDefinition) (string, error) {
	for _, version := range crd.Spec.Versions {
		if version.Served && version.Storage {
			return version.Name, nil
		}
	}
	return "", errors.New("no served and storage version found in CustomResourceDefinition")
}

// GetXRDVersion iterates over the versions defined in the CompositeResourceDefinition (XRD).
// It returns the name of the version that has both "served" and "referenceable" fields set to true.
func GetXRDVersion(xrd v1.CompositeResourceDefinition) (string, error) {
	for _, version := range xrd.Spec.Versions {
		if version.Served && version.Referenceable {
			return version.Name, nil
		}
	}
	return "", errors.New("no referenceable version found in CompositeResourceDefinition")
}
