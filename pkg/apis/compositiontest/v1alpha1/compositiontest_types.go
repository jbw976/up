// Copyright 2025 Upbound Inc.
// All rights reserved

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// CompositionTest defines the schema for the CompositionTest custom resource.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=comptest,categories=meta
type CompositionTest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec CompositionTestSpec `json:"spec"`
}

// CompositionTestSpec defines the specification for the CompositionTest custom resource.
//
// +k8s:deepcopy-gen=true
type CompositionTestSpec struct {
	// Timeout for the test in seconds
	// Required. Default is 30s.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=30
	TimeoutSeconds int `json:"timeoutSeconds"`

	// Validate indicates whether to validate managed resources against schemas.
	// Optional.
	// +kubebuilder:validation:Optional
	Validate *bool `json:"validate,omitempty"`

	// XR specifies the composite resource (XR) inline.
	XR runtime.RawExtension `json:"xr,omitempty"`

	// XR specifies the composite resource (XR) path.
	// Optional.
	XRPath string `json:"xrPath,omitempty"`

	// XRD specifies the XRD definition inline.
	// Optional.
	XRD runtime.RawExtension `json:"xrd,omitempty"`

	// XRD specifies the XRD definition path.
	// Optional.
	XRDPath string `json:"xrdPath,omitempty"`

	// Composition specifies the composition definition inline.
	// Optional.
	Composition runtime.RawExtension `json:"composition,omitempty"`

	// Composition specifies the composition definition path.
	// Optional.
	CompositionPath string `json:"compositionPath,omitempty"`

	// ObservedResources specifies additional observed resources inline or path.
	// Optional.
	// +kubebuilder:validation:Optional
	ObservedResources []runtime.RawExtension `json:"observedResources,omitempty"`

	// ExtraResources specifies additional resources inline or path.
	// Optional.
	// +kubebuilder:validation:Optional
	ExtraResources []runtime.RawExtension `json:"extraResources,omitempty"`

	// Context specifies context for the Function Pipeline inline or path.
	// Optional.
	// +kubebuilder:validation:Optional
	Context []runtime.RawExtension `json:"context,omitempty"`

	// AssertResources defines assertions to validate resources after test completion.
	// Optional.
	// +kubebuilder:validation:Optional
	AssertResources []runtime.RawExtension `json:"assertResources,omitempty"`
}
