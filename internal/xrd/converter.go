// Copyright 2025 Upbound Inc.
// All rights reserved

// Package xrd handled xrd related functions
package xrd

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/apis/apiextensions/v2alpha1"
)

// ConvertV2Alpha1ToV1 converts a v2alpha1 XRD to v1 XRD format for compatibility with existing xcrd functions.
func ConvertV2Alpha1ToV1(v2XRD *v2alpha1.CompositeResourceDefinition) *v1.CompositeResourceDefinition {
	v1XRD := &v1.CompositeResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1.CompositeResourceDefinitionGroupVersionKind.GroupVersion().String(),
			Kind:       v1.CompositeResourceDefinitionGroupVersionKind.Kind,
		},
		ObjectMeta: v2XRD.ObjectMeta,
		Spec: v1.CompositeResourceDefinitionSpec{
			Group:                          v2XRD.Spec.Group,
			Names:                          v2XRD.Spec.Names,
			DefaultCompositionUpdatePolicy: v2XRD.Spec.DefaultCompositionUpdatePolicy,
			Conversion:                     v2XRD.Spec.Conversion,
		},
	}

	// Convert scope from v2alpha1 to v1
	switch v2XRD.Spec.Scope {
	case v2alpha1.CompositeResourceScopeNamespaced:
		scope := v1.CompositeResourceScopeNamespaced
		v1XRD.Spec.Scope = &scope
	case v2alpha1.CompositeResourceScopeCluster:
		scope := v1.CompositeResourceScopeCluster
		v1XRD.Spec.Scope = &scope
	default:
		// Default to cluster scoped
		scope := v1.CompositeResourceScopeCluster
		v1XRD.Spec.Scope = &scope
	}

	// Convert versions
	v1XRD.Spec.Versions = make([]v1.CompositeResourceDefinitionVersion, len(v2XRD.Spec.Versions))
	for i, v2Version := range v2XRD.Spec.Versions {
		v1XRD.Spec.Versions[i] = v1.CompositeResourceDefinitionVersion{
			Name:          v2Version.Name,
			Served:        v2Version.Served,
			Referenceable: v2Version.Referenceable,
			Deprecated:    v2Version.Deprecated,
			Schema:        (*v1.CompositeResourceValidation)(v2Version.Schema),
		}
	}

	return v1XRD
}
