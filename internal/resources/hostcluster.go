// Copyright 2025 Upbound Inc.
// All rights reserved

package resources

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
)

// HostCluster represents the HostCluster CustomResource and extends an
// unstructured.Unstructured.
type HostCluster struct {
	unstructured.Unstructured
}

// GetUnstructured returns the underlying *unstructured.Unstructured.
func (h *HostCluster) GetUnstructured() *unstructured.Unstructured {
	return &h.Unstructured
}

// GetCondition returns the condition for the given xpv1.ConditionType if it
// exists, otherwise returns nil.
func (h *HostCluster) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	conditioned := xpv1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	if err := fieldpath.Pave(h.Object).GetValueInto("status", &conditioned); err != nil {
		return xpv1.Condition{}
	}
	return conditioned.GetCondition(ct)
}

// SetCompositionSelector of this composite resource claim.
func (h *HostCluster) SetCompositionSelector(sel *metav1.LabelSelector) {
	_ = fieldpath.Pave(h.Object).SetValue("spec.compositionSelector", sel)
}
