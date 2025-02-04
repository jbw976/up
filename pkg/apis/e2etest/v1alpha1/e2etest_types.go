// Copyright 2025 The Upbound Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"

	"github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
)

// E2ETest defines the schema for the E2ETest custom resource.
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

// E2ETestSpec defines the specification for the E2ETest custom resource.
//
// +k8s:deepcopy-gen=true
// +kubebuilder:validation:Required
type E2ETestSpec struct {
	// Crossplane configuration for the test
	// +kubebuilder:validation:Required
	Crossplane *v1beta1.CrossplaneSpec `json:"crossplane,omitempty"`

	// Timeout for the test in seconds
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=1
	TimeoutSeconds *int `json:"timeoutSeconds,omitempty"`

	// If true, skip resource deletion after test
	// +kubebuilder:validation:Optional
	SkipDelete *bool `json:"skipDelete,omitempty"`

	// Default conditions for the test
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MinItems=1
	DefaultConditions []string `json:"defaultConditions,omitempty"`

	// Required manifests for the test
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Manifests []runtime.RawExtension `json:"manifests"`

	// Additional resources for the test
	// +kubebuilder:validation:Optional
	ExtraResources []runtime.RawExtension `json:"extraResources,omitempty"`
}
