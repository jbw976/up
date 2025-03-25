// Copyright 2025 Upbound Inc.
// All rights reserved

package simulation

import (
	"context"
	"fmt"
	"slices"

	diffv3 "github.com/r3labs/diff/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	xpextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	xpv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	xpv1beta1 "github.com/crossplane/crossplane/apis/pkg/v1beta1"

	spacesv1alpha1 "github.com/upbound/up-sdk-go/apis/spaces/v1alpha1"
	"github.com/upbound/up/internal/diff"
	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/upbound"
)

const (
	// annotationKeyClonedState is the annotation key storing the JSON
	// representation of the state of the resource at control plane clone time.
	annotationKeyClonedState = "simulation.spaces.upbound.io/cloned-state"
)

// DiffSet connects to the simulated control plane and calculates each of the
// changes within every resource marked as updated in the status of the
// simulation.
func (r *Run) DiffSet(ctx context.Context, upCtx *upbound.Context, excluded []schema.GroupKind) ([]diff.ResourceDiff, error) {
	if len(r.diffSet) > 0 {
		return r.diffSet, nil
	}
	config, err := r.RESTConfig(ctx, upCtx)
	if err != nil {
		return []diff.ResourceDiff{}, errors.Wrap(err, "unable to connect to simulated control plane")
	}
	r.diffSet, err = r.createResourceDiffSet(ctx, config, r.simulation.Status.Changes, excluded)
	if err != nil {
		return []diff.ResourceDiff{}, errors.Wrap(err, "error while creating resource diff set")
	}
	return r.diffSet, nil
}

// removeFieldsForDiff removes any fields that should be excluded from the diff.
func removeFieldsForDiff(u *unstructured.Unstructured) error {
	// based on the filters in the simulation preprocessor
	// https://github.com/upbound/spaces/blob/v1.8.0/internal/controller/mxe/simulation/preprocess.go#L100-L108
	trim := []string{
		fmt.Sprintf("metadata.annotations['%s']", annotationKeyClonedState),
		"metadata.annotations['kubectl.kubernetes.io/last-applied-configuration']",
		"metadata.creationTimestamp",
		"metadata.finalizers",
		"metadata.generateName",
		"metadata.generation",
		"metadata.managedFields",
		"metadata.ownerReferences",
		"metadata.resourceVersion",
		"metadata.uid",
		"spec.compositionRevisionRef",
	}

	wildcards := []string{
		"status.conditions[*].lastTransitionTime",
	}

	p := fieldpath.Pave(u.UnstructuredContent())

	// expand each wildcard path and add to list to trim
	for _, wildcard := range wildcards {
		expanded, err := p.ExpandWildcards(wildcard)
		if err != nil {
			return errors.Wrap(err, "unable to expand wildcards in ignored fields")
		}
		trim = append(trim, expanded...)
	}

	for _, path := range trim {
		if err := p.DeleteField(path); err != nil {
			return errors.Wrap(err, "cannot delete field")
		}
	}

	return nil
}

// createResourceDiffSet reads through all of the changes from the simulation
// status and looks up the difference between the initial version of the
// resource and the version currently in the API server (at the time of the
// function call).
func (r *Run) createResourceDiffSet(ctx context.Context, config *rest.Config, changes []spacesv1alpha1.SimulationChange, excluded []schema.GroupKind) ([]diff.ResourceDiff, error) { //nolint:gocyclo // TODO: simplify this
	lookup, err := kube.NewDiscoveryResourceLookup(config)
	if err != nil {
		return []diff.ResourceDiff{}, errors.Wrap(err, "unable to create resource lookup client")
	}

	dyn, err := dynamic.NewForConfig(config)
	if err != nil {
		return []diff.ResourceDiff{}, errors.Wrap(err, "unable to create dynamic client")
	}

	diffSet := make([]diff.ResourceDiff, 0, len(changes))

	r.debugPrintf("iterating over %d changes\n", len(changes))

	// the set of types that should be excluded from the diff set by
	// default, which are not already being excluded by the reconciler.
	allExcluded := []schema.GroupKind{
		xpextensionsv1.CompositionRevisionGroupVersionKind.GroupKind(),
		xpv1.ConfigurationRevisionGroupVersionKind.GroupKind(),
		xpv1.FunctionGroupVersionKind.GroupKind(),
		xpv1.FunctionRevisionGroupVersionKind.GroupKind(),
		xpv1beta1.DeploymentRuntimeConfigGroupVersionKind.GroupKind(),
		xpv1beta1.LockGroupVersionKind.GroupKind(),
	}
	allExcluded = append(allExcluded, excluded...)

	for _, change := range changes {
		gvk := schema.FromAPIVersionAndKind(change.ObjectReference.APIVersion, change.ObjectReference.Kind)

		// todo(redbackthomson): Remove this logic once we have done a better
		// job of filtering in the reconciler
		if slices.Contains(allExcluded, gvk.GroupKind()) {
			r.debugPrintf("skipping gvk %+v\n", gvk)
			continue
		}

		rs, err := lookup.Get(gvk)
		if err != nil {
			r.debugPrintf("unable to find gvk from lookup %q\n", gvk)
			return []diff.ResourceDiff{}, err
		}

		switch change.Change { //nolint:exhaustive // Proceed with the rest of the loop if other.
		case spacesv1alpha1.SimulationChangeTypeCreate:
			diffSet = append(diffSet, diff.ResourceDiff{
				SimulationChange: change,
			})
			r.debugPrintf("appended create to diff set for %v\n", change.ObjectReference)
			continue
		case spacesv1alpha1.SimulationChangeTypeDelete:
			diffSet = append(diffSet, diff.ResourceDiff{
				SimulationChange: change,
			})
			r.debugPrintf("appended delete to diff set for %v\n", change.ObjectReference)
			continue
		}

		var cl dynamic.ResourceInterface
		ncl := dyn.Resource(schema.GroupVersionResource{
			Group:    rs.Group,
			Version:  rs.Version,
			Resource: rs.Name,
		})
		if change.ObjectReference.Namespace != nil {
			cl = ncl.Namespace(*change.ObjectReference.Namespace)
		} else {
			cl = ncl
		}

		after, err := cl.Get(ctx, change.ObjectReference.Name, metav1.GetOptions{})
		if err != nil {
			return []diff.ResourceDiff{}, errors.Wrap(err, "unable to get object from simulated control plane")
		}

		beforeRaw, ok := after.GetAnnotations()[annotationKeyClonedState]
		if !ok {
			r.debugPrintf("object %v is missing the previous cloned state annotation\n", change.ObjectReference)
			continue
		}
		beforeObj, err := runtime.Decode(unstructured.UnstructuredJSONScheme, []byte(beforeRaw))
		if err != nil {
			return []diff.ResourceDiff{}, errors.Wrapf(err, "previous cloned state annotation on %v could not be decoded", change.ObjectReference)
		}

		before, ok := beforeObj.(*unstructured.Unstructured)
		if !ok {
			return []diff.ResourceDiff{}, errors.Wrap(err, "before object not unstructured")
		}
		if err := removeFieldsForDiff(after); err != nil {
			return []diff.ResourceDiff{}, errors.Wrapf(err, "unable to remove fields before diff")
		}

		if err := removeFieldsForDiff(before); err != nil {
			return []diff.ResourceDiff{}, errors.Wrapf(err, "unable to remove fields before diff")
		}

		diffd, err := diffv3.Diff(before.UnstructuredContent(), after.UnstructuredContent())
		if err != nil {
			return []diff.ResourceDiff{}, errors.Wrapf(err, "unable to calculate diff for object %v", change.ObjectReference)
		}

		// we filtered out all of the changes
		if len(diffd) == 0 {
			continue
		}

		diffSet = append(diffSet, diff.ResourceDiff{
			SimulationChange: change,
			Diff:             diffd,
		})
		r.debugPrintf("appended update to diff set for %v\n", change.ObjectReference)
	}
	return diffSet, nil
}
