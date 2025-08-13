// Copyright 2025 Upbound Inc.
// All rights reserved

// Package resources supports Spaces custom resources.
package resources

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
)

// Package represents a Crossplane Package.
type Package struct {
	unstructured.Unstructured
}

// GetUnstructured returns the unstructured representation of the package.
func (p *Package) GetUnstructured() *unstructured.Unstructured {
	return &p.Unstructured
}

// GetInstalled checks whether a package is installed. If installation status
// cannot be determined, false is always returned.
func (p *Package) GetInstalled() bool {
	conditioned := xpv1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	if err := fieldpath.Pave(p.Object).GetValueInto("status", &conditioned); err != nil {
		return false
	}
	return resource.IsConditionTrue(conditioned.GetCondition("Installed"))
}

// GetHealthy checks whether a package is healhty. If health cannot be
// determined, false is always returned.
func (p *Package) GetHealthy() bool {
	conditioned := xpv1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	if err := fieldpath.Pave(p.Object).GetValueInto("status", &conditioned); err != nil {
		return false
	}
	return resource.IsConditionTrue(conditioned.GetCondition("Healthy"))
}

// SetPackage sets the package reference.
func (p *Package) SetPackage(pkg string) {
	_ = fieldpath.Pave(p.Object).SetValue("spec.package", pkg)
}
