// Copyright 2025 Upbound Inc.
// All rights reserved

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// OperationTest defines the schema for the OperationTest custom resource.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=optest,categories=meta
type OperationTest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec OperationTestSpec `json:"spec"`
}

// OperationTestSpec defines the specification for the OperationTest custom resource.
//
// +k8s:deepcopy-gen=true
type OperationTestSpec struct {
	// Timeout for the test in seconds
	// Required. Default is 30s.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=30
	TimeoutSeconds int `json:"timeoutSeconds"`

	// Operation specifies the Operation definition inline.
	// Optional.
	Operation runtime.RawExtension `json:"operation,omitempty"`

	// OperationPath specifies the XRD definition path.
	// Optional.
	OperationPath string `json:"operationPath,omitempty"`

	// RequiredResources specifies additional required resources inline.
	// Optional.
	// +kubebuilder:validation:Optional
	RequiredResources []runtime.RawExtension `json:"requiredResources,omitempty"`

	// RequiredResourcesPath specifies a path to required resources file.
	// Optional.
	// +kubebuilder:validation:Optional
	RequiredResourcesPath string `json:"requiredResourcesPath,omitempty"`

	// FunctionCredentialsPath specifies a path to a credentials file to be passed to tests.
	// Optional.
	// +kubebuilder:validation:Optional
	FunctionCredentialsPath string `json:"functionCredentialsPath,omitempty"`

	// Context specifies context for the Function Pipeline inline as key-value pairs.
	// Keys are context keys, values are JSON data.
	// Optional.
	// +kubebuilder:validation:Optional
	Context map[string]runtime.RawExtension `json:"context,omitempty"`

	// AssertResources defines assertions to validate resources after test completion.
	// Optional.
	// +kubebuilder:validation:Optional
	AssertResources []runtime.RawExtension `json:"assertResources,omitempty"`
}
