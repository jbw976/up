// Copyright 2025 Upbound Inc.
// All rights reserved

package upbound

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	upboundv1alpha1 "github.com/upbound/up-sdk-go/apis/upbound/v1alpha1"
)

// DisconnectedConfiguration is the configuration for a disconnected space.
type DisconnectedConfiguration struct {
	HubContext string `json:"hubContext"`
}

// CloudConfiguration is the configuration of a cloud space.
type CloudConfiguration struct {
	Organization string `json:"organization"`
	SpaceName    string `json:"space"`
}

// SpaceExtensionSpec is the spec of SpaceExtension
//
// +k8s:deepcopy-gen=true
type SpaceExtensionSpec struct {
	Disconnected *DisconnectedConfiguration `json:"disconnected,omitempty"`
	Cloud        *CloudConfiguration        `json:"cloud,omitempty"`
}

// SpaceExtension is a kubeconfig context extension that defines metadata about
// a space context
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type SpaceExtension struct {
	metav1.TypeMeta `json:",inline"`

	Spec *SpaceExtensionSpec `json:"spec,omitempty"`
}

// SpaceExtensionKind is kind of SpaceExtension.
var SpaceExtensionKind = reflect.TypeOf(SpaceExtension{}).Name()

func NewCloudV1Alpha1SpaceExtension(org, space string) *SpaceExtension {
	return &SpaceExtension{
		TypeMeta: metav1.TypeMeta{
			Kind:       SpaceExtensionKind,
			APIVersion: upboundv1alpha1.SchemeGroupVersion.String(),
		},
		Spec: &SpaceExtensionSpec{
			Cloud: &CloudConfiguration{
				Organization: org,
				SpaceName:    space,
			},
		},
	}
}

func NewDisconnectedV1Alpha1SpaceExtension(hubContext string) *SpaceExtension {
	return &SpaceExtension{
		TypeMeta: metav1.TypeMeta{
			Kind:       SpaceExtensionKind,
			APIVersion: "spaces.upbound.io/v1alpha1",
		},
		Spec: &SpaceExtensionSpec{
			Disconnected: &DisconnectedConfiguration{
				HubContext: hubContext,
			},
		},
	}
}
