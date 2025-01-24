// Copyright 2025 Upbound Inc.
// All rights reserved

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

const (
	// Group is the API Group for projects.
	Group = "meta.dev.upbound.io"
	// Version is the API version for projects.
	Version = "v1alpha1"
	// GroupVersion is the GroupVersion for projects.
	GroupVersion = Group + "/" + Version
	// E2ETestKind is the kind of a Project.
	E2ETestKind = "E2ETest"
)

var (
	// SchemeGroupVersion is group version used to register these objects.
	SchemeGroupVersion = schema.GroupVersion{Group: Group, Version: Version}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = &scheme.Builder{GroupVersion: SchemeGroupVersion}

	// AddToScheme adds all registered types to scheme.
	AddToScheme = SchemeBuilder.AddToScheme

	// E2ETestGroupVersionKind adds SchemeGroupVersion.
	E2ETestGroupVersionKind = SchemeGroupVersion.WithKind(E2ETestKind)
)

func init() {
	SchemeBuilder.Register(&E2ETest{})
}
