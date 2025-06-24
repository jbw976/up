// Copyright 2025 Upbound Inc.
// All rights reserved

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"

	"github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
)

// E2ETest defines the schema for the E2ETest custom resource used for e2e
// testing of Crossplane configurations in controlplanes.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=e2e,categories=meta
type E2ETest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec E2ETestSpec `json:"spec"`
}

// E2ETestSpec defines the specification for e2e testing of Crossplane
// configurations. It orchestrates the complete test lifecycle including setting
// up controlplane, applying test resources in the correct order (InitResources
// → Configuration → ExtraResources → Manifests), validating conditions, and
// handling cleanup. This spec allows you to define e2e tests that verify your
// Crossplane compositions, providers, and managed resources work correctly
// together in a real controlplane environment.
//
// +k8s:deepcopy-gen=true
// +kubebuilder:validation:Required
type E2ETestSpec struct {
	// Crossplane specifies the Crossplane configuration and settings required
	// for this test. This includes the version of Universal Crossplane to
	// install, and optional auto-upgrade settings. The configuration defined
	// here will be used to set up the controlplane before applying the test
	// manifests.
	// +kubebuilder:validation:Required
	Crossplane *v1beta1.CrossplaneSpec `json:"crossplane,omitempty"`

	// TimeoutSeconds defines the maximum duration in seconds that the test is
	// allowed to run before being marked as failed. This includes time for
	// resource creation, condition checks, and any reconciliation processes. If
	// not specified, a default timeout will be used. Consider setting higher
	// values for tests involving complex resources or those requiring multiple
	// reconciliation cycles.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=1
	TimeoutSeconds *int `json:"timeoutSeconds,omitempty"`

	// If true, skip resource deletion after test
	// +kubebuilder:validation:Optional
	SkipDelete *bool `json:"skipDelete,omitempty"`

	// DefaultConditions specifies the expected conditions that should be met
	// after the manifests are applied. These are validation checks that verify
	// the resources are functioning correctly. Each condition is a string
	// expression that will be evaluated against the deployed resources. Common
	// conditions include checking resource status for readiness
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MinItems=1
	DefaultConditions []string `json:"defaultConditions,omitempty"`

	// Manifests contains the Kubernetes resources that will be applied as part
	// of this e2e test. These are the primary resources being tested - they
	// will be created in the controlplane and then validated against the
	// conditions specified in DefaultConditions. Each manifest must be a valid
	// Kubernetes object. At least one manifest is required. Examples include
	// Claims, Composite Resources or any Kubernetes resource you want to test.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Manifests []runtime.RawExtension `json:"manifests"`

	// ExtraResources specifies additional Kubernetes resources that should be
	// created or updated after the configuration has been successfully applied.
	// These resources may depend on the primary configuration being in place.
	// Common use cases include ConfigMaps, Secrets, providerConfigs. Each
	// resource must be a valid Kubernetes object.
	// +kubebuilder:validation:Optional
	ExtraResources []runtime.RawExtension `json:"extraResources,omitempty"`

	// InitResources specifies Kubernetes resources that must be created or
	// updated before the configuration is applied. These are typically
	// prerequisite resources that the configuration depends on. Common use
	// cases include ImageConfigs, DeploymentRuntimeConfigs, or any foundational
	// resources required for the configuration to work. Each resource must be a
	// valid Kubernetes object.
	// +kubebuilder:validation:Optional
	InitResources []runtime.RawExtension `json:"initResources,omitempty"`
}
