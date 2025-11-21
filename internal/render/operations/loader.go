// Copyright 2025 Upbound Inc.
// All rights reserved

package operations

import (
	"bytes"
	"fmt"

	"github.com/spf13/afero"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/utils/ptr"
	kyaml "sigs.k8s.io/yaml"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	v1alpha1 "github.com/crossplane/crossplane/v2/apis/ops/v1alpha1"
	oprender "github.com/crossplane/crossplane/v2/cmd/crank/alpha/render/op"
)

func loadOperationWithTemplateSupport(fs afero.Fs, path string) (*v1alpha1.Operation, error) {
	data, err := afero.ReadFile(fs, path)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot read operation file %q", path)
	}

	// First, check what kind of resource this is
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 1024)
	var meta struct {
		APIVersion string `json:"apiVersion"`
		Kind       string `json:"kind"`
	}
	if err := decoder.Decode(&meta); err != nil {
		return nil, errors.Wrapf(err, "cannot decode resource metadata from %q", path)
	}

	switch {
	case meta.APIVersion == "ops.crossplane.io/v1alpha1" && meta.Kind == "Operation":
		// Regular Operation - use the existing LoadOperation function
		return oprender.LoadOperation(fs, path)

	case meta.APIVersion == "ops.crossplane.io/v1alpha1" && meta.Kind == "CronOperation":
		// Parse as CronOperation and extract the template
		var cronOp v1alpha1.CronOperation
		if err := kyaml.Unmarshal(data, &cronOp); err != nil {
			return nil, errors.Wrapf(err, "cannot unmarshal CronOperation from %q", path)
		}
		return newOperationFromCronOperation(&cronOp), nil

	case meta.APIVersion == "ops.crossplane.io/v1alpha1" && meta.Kind == "WatchOperation":
		// Parse as WatchOperation and extract the template
		var watchOp v1alpha1.WatchOperation
		if err := kyaml.Unmarshal(data, &watchOp); err != nil {
			return nil, errors.Wrapf(err, "cannot unmarshal WatchOperation from %q", path)
		}
		return newOperationFromWatchOperation(&watchOp), nil

	default:
		return nil, errors.Errorf("unsupported resource kind %q with apiVersion %q in %q. Supported kinds: Operation, CronOperation, WatchOperation", meta.Kind, meta.APIVersion, path)
	}
}

// newOperationFromCronOperation creates a new Operation from a CronOperation's template.
func newOperationFromCronOperation(co *v1alpha1.CronOperation) *v1alpha1.Operation {
	op := &v1alpha1.Operation{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
			Kind:       v1alpha1.OperationKind,
		},
		ObjectMeta: co.Spec.OperationTemplate.ObjectMeta,
		Spec:       co.Spec.OperationTemplate.Spec,
	}

	// Generate a name if not provided in the template
	if op.GetName() == "" {
		// Name the operation the same as the CronOperation.
		op.SetName(co.GetName())
	}

	return op
}

// newOperationFromWatchOperation creates a new Operation from a WatchOperation's template,
// injecting the watched resource into all pipeline steps.
// ToDo(haarchri): need to check how the resource gets added to the Operation (requiredResourceSelector)
// - do we need an Annotation on the RequiredResource?
func newOperationFromWatchOperation(wo *v1alpha1.WatchOperation) *v1alpha1.Operation {
	// Deep copy the spec to avoid mutating the original template
	spec := wo.Spec.OperationTemplate.Spec.DeepCopy()

	// Create a placeholder watched resource from the watch spec
	// In the actual runtime, this would be the real watched resource
	watched := &unstructured.Unstructured{}
	watched.SetAPIVersion(wo.Spec.Watch.APIVersion)
	watched.SetKind(wo.Spec.Watch.Kind)

	// Use a placeholder name for CLI rendering
	// In runtime, this would be the actual resource name
	watchedName := "watched-resource"
	watched.SetName(watchedName)

	// Set namespace if specified
	if wo.Spec.Watch.Namespace != "" {
		watched.SetNamespace(wo.Spec.Watch.Namespace)
	}

	sel := v1alpha1.RequiredResourceSelector{
		RequirementName: v1alpha1.RequirementNameWatchedResource,
		APIVersion:      watched.GetAPIVersion(),
		Kind:            watched.GetKind(),
		// ToDo(haarchri): do we need to read the name from the RequiredResource and then set it here ?!
		Name: ptr.To(watched.GetName()),
	}

	// Add namespace if the resource is namespaced
	if watched.GetNamespace() != "" {
		sel.Namespace = ptr.To(watched.GetNamespace())
	}

	// Inject the watched resource into each pipeline step
	for i := range spec.Pipeline {
		step := &spec.Pipeline[i]

		if step.Requirements == nil {
			step.Requirements = &v1alpha1.FunctionRequirements{}
		}

		step.Requirements.RequiredResources = append(step.Requirements.RequiredResources, sel)
	}

	op := &v1alpha1.Operation{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
			Kind:       v1alpha1.OperationKind,
		},
		ObjectMeta: wo.Spec.OperationTemplate.ObjectMeta,
		Spec:       *spec,
	}

	// Generate a name for the operation
	var name string
	if op.GetName() != "" {
		name = op.GetName()
	} else {
		// For CLI rendering, use WatchOperation name with suffix
		name = fmt.Sprintf("%s-rendered", wo.GetName())
	}
	op.SetName(name)

	meta.AddLabels(op, map[string]string{v1alpha1.LabelWatchOperationName: wo.GetName()})

	// Add annotations with information about the watched resource
	annotations := map[string]string{
		v1alpha1.AnnotationWatchedResourceAPIVersion:      watched.GetAPIVersion(),
		v1alpha1.AnnotationWatchedResourceKind:            watched.GetKind(),
		v1alpha1.AnnotationWatchedResourceName:            watched.GetName(),
		v1alpha1.AnnotationWatchedResourceResourceVersion: watched.GetResourceVersion(),
	}

	// Add namespace annotation if the resource is namespaced
	if watched.GetNamespace() != "" {
		annotations[v1alpha1.AnnotationWatchedResourceNamespace] = watched.GetNamespace()
	}

	meta.AddAnnotations(op, annotations)

	av, k := v1alpha1.WatchOperationGroupVersionKind.ToAPIVersionAndKind()
	meta.AddOwnerReference(op, meta.AsController(&xpv1.TypedReference{
		APIVersion: av,
		Kind:       k,
		Name:       wo.GetName(),
		UID:        wo.GetUID(),
	}))

	return op
}
