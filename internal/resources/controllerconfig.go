// Copyright 2025 Upbound Inc.
// All rights reserved

package resources

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
)

// ControllerConfigGRV is the GroupVersionResource used for
// the Crossplane ControllerConfig.
var ControllerConfigGRV = schema.GroupVersionResource{
	Group:    "pkg.crossplane.io",
	Version:  "v1alpha1",
	Resource: "controllerconfigs",
}

// ControllerConfig represents a Crossplane ControllerConfig.
type ControllerConfig struct {
	unstructured.Unstructured
}

// GetUnstructured returns the unstructured representation of the package.
func (c *ControllerConfig) GetUnstructured() *unstructured.Unstructured {
	return &c.Unstructured
}

// SetServiceAccountName for the ControllerConfig.
func (c *ControllerConfig) SetServiceAccountName(name string) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.serviceAccountName", name)
}
