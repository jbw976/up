// Copyright 2025 Upbound Inc.
// All rights reserved

package function

import (
	"encoding/json"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	apiextv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	opsv1alpha1 "github.com/crossplane/crossplane/apis/ops/v1alpha1"
)

type pipeline interface {
	addStep(name, functionRef string)
	json.Marshaler
}

type compositionPipeline struct {
	wrap *apiextv1.Composition
}

func (c *compositionPipeline) addStep(name, functionRef string) {
	step := apiextv1.PipelineStep{
		Step: name,
		FunctionRef: apiextv1.FunctionReference{
			Name: functionRef,
		},
	}

	for _, existingStep := range c.wrap.Spec.Pipeline {
		if existingStep.Step == step.Step && existingStep.FunctionRef.Name == step.FunctionRef.Name {
			// Step already exists, no need to add it
			return
		}
	}

	c.wrap.Spec.Pipeline = append([]apiextv1.PipelineStep{step}, c.wrap.Spec.Pipeline...)
}

func (c *compositionPipeline) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.wrap)
}

type operationPipeline struct {
	// wrap is an OperationSpec so we can work with CronOperations and
	// WatchOperations as well as oneshot Operations.
	wrap *opsv1alpha1.OperationSpec

	// parent is the parent that contains the pipeline, used for marshaling.
	parent runtime.Object
}

func (o *operationPipeline) addStep(name, functionRef string) {
	step := opsv1alpha1.PipelineStep{
		Step: name,
		FunctionRef: opsv1alpha1.FunctionReference{
			Name: functionRef,
		},
	}

	for _, existingStep := range o.wrap.Pipeline {
		if existingStep.Step == step.Step && existingStep.FunctionRef.Name == step.FunctionRef.Name {
			// Step already exists, no need to add it
			return
		}
	}

	// If there's only one step and it's function-dummy, remove it. This is the
	// default pipeline we generate in `up operation generate`, but
	// function-dummy doesn't do anything useful.
	if len(o.wrap.Pipeline) == 1 && o.wrap.Pipeline[0].FunctionRef.Name == "crossplane-contrib-function-dummy" {
		o.wrap.Pipeline = make([]opsv1alpha1.PipelineStep, 0, 1)
	}

	o.wrap.Pipeline = append([]opsv1alpha1.PipelineStep{step}, o.wrap.Pipeline...)
}

func (o *operationPipeline) MarshalJSON() ([]byte, error) {
	return json.Marshal(o.parent)
}

func convertToPipeline(u *unstructured.Unstructured) (pipeline, error) {
	switch u.GroupVersionKind().String() {
	case apiextv1.CompositionGroupVersionKind.String():
		var c apiextv1.Composition
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &c); err != nil {
			return nil, err
		}

		return &compositionPipeline{wrap: &c}, nil

	case opsv1alpha1.OperationGroupVersionKind.String():
		var o opsv1alpha1.Operation
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &o); err != nil {
			return nil, err
		}

		return &operationPipeline{wrap: &o.Spec, parent: &o}, nil

	case opsv1alpha1.CronOperationGroupVersionKind.String():
		var o opsv1alpha1.CronOperation
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &o); err != nil {
			return nil, err
		}

		return &operationPipeline{wrap: &o.Spec.OperationTemplate.Spec, parent: &o}, nil

	case opsv1alpha1.WatchOperationGroupVersionKind.String():
		var o opsv1alpha1.WatchOperation
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &o); err != nil {
			return nil, err
		}

		return &operationPipeline{wrap: &o.Spec.OperationTemplate.Spec, parent: &o}, nil

	default:
		return nil, errors.Errorf("unknown pipeline gvk %s", u.GroupVersionKind())
	}
}
