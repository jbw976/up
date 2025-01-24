// Copyright 2025 Upbound Inc.
// All rights reserved

package composite

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane/crossplane-runtime/pkg/meta"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

// Annotation keys.
const (
	AnnotationKeyCompositionResourceName = "crossplane.io/composition-resource-name"
)

// SetCompositionResourceName sets the name of the composition template used to
// reconcile a composed resource as an annotation.
func SetCompositionResourceName(o metav1.Object, n ResourceName) {
	meta.AddAnnotations(o, map[string]string{AnnotationKeyCompositionResourceName: string(n)})
}

// GetCompositionResourceName gets the name of the composition template used to
// reconcile a composed resource from its annotations.
func GetCompositionResourceName(o metav1.Object) string {
	return o.GetAnnotations()[AnnotationKeyCompositionResourceName]
}

// Returns types of patches that are from a composed resource _to_ a composite resource.
func patchTypesToXR() []v1.PatchType {
	return []v1.PatchType{v1.PatchTypeToCompositeFieldPath, v1.PatchTypeCombineToComposite}
}

// Returns types of patches that are _from_ a composite resource to a composed resource.
func patchTypesFromXR() []v1.PatchType {
	return []v1.PatchType{v1.PatchTypeFromCompositeFieldPath, v1.PatchTypeCombineFromComposite}
}
