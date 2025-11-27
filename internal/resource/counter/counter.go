// Copyright 2025 Upbound Inc.
// All rights reserved

// Package counter provides functionality to count Crossplane resources in a Kubernetes cluster.
package counter

import (
	"context"
	"fmt"
	"slices"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
)

const defaultPageSize = 500

// Counter counts Crossplane resources in a Kubernetes cluster.
type Counter struct {
	crdClient     apiextensionsclientset.Interface
	dynamicClient dynamic.Interface
}

// New creates a new Counter from a rest.Config.
func New(cfg *rest.Config) (*Counter, error) {
	crdClient, err := apiextensionsclientset.NewForConfig(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create CRD client")
	}

	dynamicClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create dynamic client")
	}

	return &Counter{
		crdClient:     crdClient,
		dynamicClient: dynamicClient,
	}, nil
}

// Count counts all Crossplane resources in the cluster.
func (c *Counter) Count(ctx context.Context) (*ResourceCounts, error) {
	crds, err := c.fetchAllCRDs(ctx)
	if err != nil {
		return nil, err
	}

	counts := &ResourceCounts{}
	uniqueResources := make(map[string]struct{})

	for _, crd := range crds {
		if err := c.countCRDResources(ctx, crd, counts, uniqueResources); err != nil {
			return nil, err
		}
	}

	counts.TotalResources = counts.ManagedResources + counts.CompositeResources +
		counts.CompositeResourceClaims + counts.ComposedResources

	return counts, nil
}

// countCRDResources counts resources for a single CRD.
func (c *Counter) countCRDResources(ctx context.Context, crd apiextensionsv1.CustomResourceDefinition, counts *ResourceCounts, uniqueResources map[string]struct{}) error {
	rt := classifyCRD(crd)
	if rt == resourceTypeExcluded {
		return nil
	}

	gvr, ok := getStorageGVR(crd)
	if !ok {
		return nil // Skip CRDs without a storage version
	}
	resources, err := c.listResources(ctx, gvr, crd.Spec.Scope == apiextensionsv1.NamespaceScoped)
	if err != nil {
		return errors.Wrapf(err, "cannot list resources for CRD %q", crd.Name)
	}

	for _, res := range resources {
		countResource(res, rt, counts, uniqueResources)
	}
	return nil
}

// countResource counts a single resource and its composed resources.
func countResource(res unstructured.Unstructured, rt resourceType, counts *ResourceCounts, uniqueResources map[string]struct{}) {
	key := createResourceKey(res)
	if _, exists := uniqueResources[key]; exists {
		return
	}
	uniqueResources[key] = struct{}{}

	switch rt {
	case resourceTypeExcluded:
		// Skip excluded resources
	case resourceTypeManagedResource:
		counts.ManagedResources++
	case resourceTypeComposite:
		counts.CompositeResources++
		countComposedResources(res, counts, uniqueResources)
	case resourceTypeClaim:
		counts.CompositeResourceClaims++
		countComposedResources(res, counts, uniqueResources)
	}
}

// countComposedResources extracts and counts composed resources from an XR or Claim.
func countComposedResources(res unstructured.Unstructured, counts *ResourceCounts, uniqueResources map[string]struct{}) {
	for _, composedKey := range extractComposedResourceKeys(res) {
		if _, exists := uniqueResources[composedKey]; !exists {
			uniqueResources[composedKey] = struct{}{}
			counts.ComposedResources++
		}
	}
}

// fetchAllCRDs retrieves all CRDs from the cluster with pagination.
func (c *Counter) fetchAllCRDs(ctx context.Context) ([]apiextensionsv1.CustomResourceDefinition, error) {
	var crds []apiextensionsv1.CustomResourceDefinition

	continueToken := ""
	for {
		l, err := c.crdClient.ApiextensionsV1().CustomResourceDefinitions().List(ctx, metav1.ListOptions{
			Limit:    defaultPageSize,
			Continue: continueToken,
		})
		if err != nil {
			return nil, errors.Wrap(err, "cannot list CRDs")
		}
		crds = append(crds, l.Items...)
		continueToken = l.GetContinue()
		if continueToken == "" {
			break
		}
	}

	return crds, nil
}

// listResources lists all resources for a given GVR with pagination.
func (c *Counter) listResources(ctx context.Context, gvr schema.GroupVersionResource, namespaced bool) ([]unstructured.Unstructured, error) {
	var resources []unstructured.Unstructured

	continueToken := ""
	for {
		var l *unstructured.UnstructuredList
		var err error

		if namespaced {
			l, err = c.dynamicClient.Resource(gvr).Namespace("").List(ctx, metav1.ListOptions{
				Limit:    defaultPageSize,
				Continue: continueToken,
			})
		} else {
			l, err = c.dynamicClient.Resource(gvr).List(ctx, metav1.ListOptions{
				Limit:    defaultPageSize,
				Continue: continueToken,
			})
		}
		if err != nil {
			return nil, err
		}

		resources = append(resources, l.Items...)
		continueToken = l.GetContinue()
		if continueToken == "" {
			break
		}
	}

	return resources, nil
}

// classifyCRD determines the type of a CRD based on its owner references.
func classifyCRD(crd apiextensionsv1.CustomResourceDefinition) resourceType {
	for _, ref := range crd.OwnerReferences {
		if strings.HasPrefix(ref.APIVersion, "pkg.crossplane.io") && ref.Kind == "ProviderRevision" {
			if isProviderConfig(crd.Spec.Names.Kind) {
				return resourceTypeExcluded
			}
			return resourceTypeManagedResource
		}
		if strings.HasPrefix(ref.APIVersion, "apiextensions.crossplane.io") && ref.Kind == "CompositeResourceDefinition" {
			if slices.Contains(crd.Spec.Names.Categories, "claim") {
				return resourceTypeClaim
			}
			return resourceTypeComposite
		}
	}
	return resourceTypeExcluded
}

// isProviderConfig checks if a kind is a ProviderConfig type.
func isProviderConfig(kind string) bool {
	return kind == "ProviderConfig" ||
		kind == "ClusterProviderConfig" ||
		kind == "ProviderConfigUsage"
}

// getStorageGVR returns the storage version GVR for a CRD.
// Returns false if no storage version is found.
func getStorageGVR(crd apiextensionsv1.CustomResourceDefinition) (schema.GroupVersionResource, bool) {
	var storageVersion string
	for _, v := range crd.Spec.Versions {
		if v.Storage {
			storageVersion = v.Name
			break
		}
	}
	if storageVersion == "" {
		return schema.GroupVersionResource{}, false
	}
	return schema.GroupVersionResource{
		Group:    crd.Spec.Group,
		Version:  storageVersion,
		Resource: crd.Spec.Names.Plural,
	}, true
}

// createResourceKey creates a unique key for a resource.
func createResourceKey(res unstructured.Unstructured) string {
	apiVersion := res.GetAPIVersion()
	kind := res.GetKind()
	namespace := res.GetNamespace()
	name := res.GetName()

	if namespace != "" {
		return fmt.Sprintf("%s/%s:%s/%s", apiVersion, kind, namespace, name)
	}
	return fmt.Sprintf("%s/%s:%s", apiVersion, kind, name)
}

// extractComposedResourceKeys extracts resource keys from spec.resourceRefs.
// Supports both Crossplane v1 (spec.resourceRefs) and v2 (spec.crossplane.resourceRefs) paths.
func extractComposedResourceKeys(res unstructured.Unstructured) []string {
	// Try Crossplane v2 path first
	resourceRefs, found, err := unstructured.NestedSlice(res.Object, "spec", "crossplane", "resourceRefs")
	if err != nil || !found {
		// Fall back to Crossplane v1 path
		resourceRefs, found, err = unstructured.NestedSlice(res.Object, "spec", "resourceRefs")
		if err != nil || !found {
			return nil
		}
	}

	keys := make([]string, 0, len(resourceRefs))
	for _, ref := range resourceRefs {
		refMap, ok := ref.(map[string]any)
		if !ok {
			continue
		}

		apiVersion, _ := refMap["apiVersion"].(string)
		kind, _ := refMap["kind"].(string)
		name, _ := refMap["name"].(string)
		namespace, _ := refMap["namespace"].(string)

		if apiVersion == "" || kind == "" || name == "" {
			continue
		}

		var key string
		if namespace != "" {
			key = fmt.Sprintf("%s/%s:%s/%s", apiVersion, kind, namespace, name)
		} else {
			key = fmt.Sprintf("%s/%s:%s", apiVersion, kind, name)
		}
		keys = append(keys, key)
	}

	if len(keys) == 0 {
		return nil
	}
	return keys
}
