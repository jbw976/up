// Copyright 2025 Upbound Inc.
// All rights reserved

package manager

import (
	"k8s.io/utils/ptr"

	metav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	metav1alpha1 "github.com/crossplane/crossplane/apis/pkg/meta/v1alpha1"
	metav1beta1 "github.com/crossplane/crossplane/apis/pkg/meta/v1beta1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
)

// ConvertToV1beta1 converts v1.Dependency types to v1beta1.Dependency types.
func ConvertToV1beta1(in metav1.Dependency) (v1beta1.Dependency, bool) {
	betaD := v1beta1.Dependency{
		Constraints: in.Version,
	}

	switch {
	case in.Provider != nil:
		betaD.Package = *in.Provider
		betaD.Type = ptr.To(v1beta1.ProviderPackageType)

	case in.Configuration != nil:
		betaD.Package = *in.Configuration
		betaD.Type = ptr.To(v1beta1.ConfigurationPackageType)

	case in.Function != nil:
		betaD.Package = *in.Function
		betaD.Type = ptr.To(v1beta1.FunctionPackageType)

	default:
		return betaD, false
	}

	return betaD, true
}

// MetaConvertToV1alpha1 converts v1.Dependency types to v1alpha1.Dependency types.
func MetaConvertToV1alpha1(in metav1.Dependency) metav1alpha1.Dependency {
	alphaD := metav1alpha1.Dependency{
		Version: in.Version,
	}
	if in.Provider != nil && in.Configuration == nil {
		alphaD.Provider = in.Provider
	}

	if in.Configuration != nil && in.Provider == nil {
		alphaD.Configuration = in.Configuration
	}

	return alphaD
}

// MetaConvertToV1beta1 converts v1.Dependency types to v1beta1.Dependency types.
func MetaConvertToV1beta1(in metav1.Dependency) metav1beta1.Dependency {
	betaD := metav1beta1.Dependency{
		Version: in.Version,
	}
	if in.Provider != nil && in.Configuration == nil {
		betaD.Provider = in.Provider
	}

	if in.Configuration != nil && in.Provider == nil {
		betaD.Configuration = in.Configuration
	}

	return betaD
}
