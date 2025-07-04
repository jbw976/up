// Copyright 2025 Upbound Inc.
// All rights reserved

package resources

import (
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
)

// ControlPlaneGVK is the GroupVersionKind used for
// provider-kubernetes ProviderConfig.
var ControlPlaneGVK = schema.GroupVersionKind{
	Group:   "spaces.upbound.io",
	Version: "v1beta1",
	Kind:    "ControlPlane",
}

// ControlPlane represents the ControlPlane CustomResource and extends an
// unstructured.Unstructured.
type ControlPlane struct {
	unstructured.Unstructured
}

// GetUnstructured returns the underlying *unstructured.Unstructured.
func (c *ControlPlane) GetUnstructured() *unstructured.Unstructured {
	c.SetGroupVersionKind(ControlPlaneGVK)
	return &c.Unstructured
}

// GetCondition returns the condition for the given xpv1.ConditionType if it
// exists, otherwise returns nil.
func (c *ControlPlane) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	conditioned := xpv1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	if err := fieldpath.Pave(c.Object).GetValueInto("status", &conditioned); err != nil {
		return xpv1.Condition{}
	}
	return conditioned.GetCondition(ct)
}

// SetConditions of this composite resource claim.
func (c *ControlPlane) SetConditions(conditions ...xpv1.Condition) {
	conditioned := xpv1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	_ = fieldpath.Pave(c.Object).GetValueInto("status", &conditioned)
	conditioned.SetConditions(conditions...)
	_ = fieldpath.Pave(c.Object).SetValue("status.conditions", conditioned.Conditions)
}

// GetControlPlaneID returns the MXP ID associated with the ControlPlane.
func (c *ControlPlane) GetControlPlaneID() string {
	id, err := fieldpath.Pave(c.Object).GetString("status.controlPlaneID")
	if err != nil {
		return ""
	}
	return id
}

// SetControlPlaneID for the MXP ID associated with the control plane.
func (c *ControlPlane) SetControlPlaneID(id string) {
	_ = fieldpath.Pave(c.Object).SetString("status.controlPlaneID", id)
}

// GetConnectionSecretToReference of this control plane.
func (c *ControlPlane) GetConnectionSecretToReference() *xpv1.SecretReference {
	out := &xpv1.SecretReference{}
	_ = fieldpath.Pave(c.Object).GetValueInto("spec.writeConnectionSecretToRef", out)
	return out
}

// SetWriteConnectionSecretToReference of this control plane.
func (c *ControlPlane) SetWriteConnectionSecretToReference(ref *xpv1.SecretReference) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.writeConnectionSecretToRef", ref)
}

func (c *ControlPlane) GetCrossplaneVersion() string {
	out, _ := fieldpath.Pave(c.Object).GetString("spec.crossplane.version")
	return out
}

func (c *ControlPlane) GetMessage() string {
	msg, err := fieldpath.Pave(c.Object).GetString("status.message")
	if err != nil {
		return ""
	}
	return msg
}

func (c *ControlPlane) SetMessage(msg string) {
	_ = fieldpath.Pave(c.Object).SetString("status.message", msg)
}

func (c *ControlPlane) GetAge() *time.Duration {
	created, _ := fieldpath.Pave(c.Object).GetString("metadata.creationTimestamp")
	if created == "" {
		return nil
	}
	ts, err := time.Parse(time.RFC3339, created)
	if err != nil {
		return nil
	}
	age := time.Since(ts)
	return &age
}
