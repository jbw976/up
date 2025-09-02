// Copyright 2025 Upbound Inc.
// All rights reserved

package ctp

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/fieldpath"

	"github.com/upbound/up/internal/async"
)

const (
	resourceStatusDeleted = "Deleted"
	resourceStatusFailed  = "Failed"
	resourceStatusPending = "Pending"
)

// CleanupOption defines functional options for cleanup behavior.
type CleanupOption func(*cleanupConfig)

type cleanupConfig struct {
	timeout      time.Duration
	eventChannel async.EventChannel
	annotation   string
}

// WithCleanupTimeout sets the total timeout for cleanup operations.
func WithCleanupTimeout(d time.Duration) CleanupOption {
	return func(cfg *cleanupConfig) {
		cfg.timeout = d
	}
}

// WithCleanupEventChannel option to set event channel.
func WithCleanupEventChannel(ch async.EventChannel) CleanupOption {
	return func(cfg *cleanupConfig) {
		cfg.eventChannel = ch
	}
}

// WithCleanupAnnotation sets the annotation to use for identifying resources to clean up.
// If not specified, defaults to "cli.upbound.io/e2etest".
func WithCleanupAnnotation(annotation string) CleanupOption {
	return func(cfg *cleanupConfig) {
		cfg.annotation = annotation
	}
}

// GenericResource represents resources.
type GenericResource struct {
	GVK          metav1.GroupVersionKind
	Name         string
	Namespace    string
	ExternalName string
	Status       string
	Message      string
}

// CleanupResult contains the results of a cleanup operation.
type CleanupResult struct {
	DeletedCount   int
	RemainingCount int
	Resources      []GenericResource
	Errors         []error
	Attempts       int
}

// cleanupHelper performs the actual cleanup logic that's common to all control plane types.
type cleanupHelper struct {
	client     client.Client
	kubeconfig clientcmd.ClientConfig
}

// Cleanup implementation.
func (h *cleanupHelper) Cleanup(ctx context.Context, opts ...CleanupOption) (*CleanupResult, error) {
	cfg := &cleanupConfig{
		timeout:    5 * time.Minute,
		annotation: "cli.upbound.io/e2etest", // default annotation
	}

	for _, opt := range opts {
		opt(cfg)
	}

	// Get discovery client first
	restConfig, err := h.kubeconfig.ClientConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get rest config")
	}

	dc, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create discovery client")
	}

	return h.performCleanup(ctx, cfg, dc)
}

// performCleanup does the cleanup.
func (h *cleanupHelper) performCleanup(ctx context.Context, cfg *cleanupConfig, dc discovery.DiscoveryInterface) (*CleanupResult, error) {
	cleanupCtx, cancel := context.WithTimeout(ctx, cfg.timeout)
	defer cancel()

	result := &CleanupResult{
		Errors: []error{},
	}

	// Find test resources to clean up
	stage := "Finding test resources"
	if cfg.eventChannel != nil {
		cfg.eventChannel.SendEvent(stage, async.EventStatusStarted)
	}

	resourceMap, err := h.findTestResources(ctx, dc, cfg.annotation)
	if err != nil {
		result.Errors = append(result.Errors, errors.Wrap(err, "failed to find test resources"))
		if cfg.eventChannel != nil {
			cfg.eventChannel.SendEvent(stage, async.EventStatusFailure)
		}
		return result, nil
	}

	if cfg.eventChannel != nil {
		cfg.eventChannel.SendEvent(stage, async.EventStatusSuccess)
	}

	initialCount := len(resourceMap)

	// Skip deletion if no resources found
	if initialCount == 0 {
		result.DeletedCount = 0
		result.RemainingCount = 0
		result.Resources = []GenericResource{}
		result.Attempts = 0
		return result, nil
	}

	// Start deletion
	stage = fmt.Sprintf("Cleanup %d test resources", initialCount)
	if cfg.eventChannel != nil {
		cfg.eventChannel.SendEvent(stage, async.EventStatusStarted)
	}

	// Deletion loop for specific resources
	result = h.runTargetedDeletionLoop(cleanupCtx, ctx, cfg, resourceMap, stage)
	return result, nil
}

// findTestResources finds all test resources that should be deleted.
func (h *cleanupHelper) findTestResources(ctx context.Context, dc discovery.DiscoveryInterface, annotation string) (map[string]GenericResource, error) {
	resourceMap := make(map[string]GenericResource)

	// Get all resources
	xpResources, err := getXPAPIResources(dc)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get API resources")
	}

	// Find claims and composites with the test annotation
	for _, gvk := range xpResources {
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   gvk.Group,
			Version: gvk.Version,
			Kind:    gvk.Kind,
		})

		// controller-runtime trims the List suffix for listing unstructured
		// resources, but we have resources like ManagedPrefixList which have
		// List in their name. We need to add another List suffix to list these
		// resources. So ManagedPrefixList becomes ManagedPrefixListList.
		if strings.HasSuffix(list.GetKind(), "List") {
			list.SetKind(gvk.Kind + "List")
		}

		if err := h.client.List(ctx, list); err != nil {
			continue
		}

		for _, item := range list.Items {
			annotations := item.GetAnnotations()
			if annotations == nil || strings.ToLower(annotations[annotation]) != "true" {
				continue
			}

			// This is a test resource - add it to deletion list
			key := fmt.Sprintf("%s/%s/%s/%s", gvk.Group, gvk.Version, gvk.Kind, item.GetName())
			resourceMap[key] = GenericResource{
				GVK:       gvk,
				Name:      item.GetName(),
				Namespace: item.GetNamespace(),
				Status:    resourceStatusPending,
				Message:   "test resource",
			}

			// Extract resource references from spec.resourceRefs or spec.crossplane.resourceRefs
			p := fieldpath.Pave(item.Object)

			if v, err := p.GetValue("spec.resourceRefs"); err == nil {
				if refs, ok := v.([]interface{}); ok {
					h.extractResourceRefs(refs, resourceMap)
				}
			}

			if v, err := p.GetValue("spec.crossplane.resourceRefs"); err == nil {
				if refs, ok := v.([]interface{}); ok {
					h.extractResourceRefs(refs, resourceMap)
				}
			}
		}
	}

	return resourceMap, nil
}

// extractResourceRefs extracts resource references and adds them to the map.
func (h *cleanupHelper) extractResourceRefs(refs []interface{}, resourceMap map[string]GenericResource) {
	for _, ref := range refs {
		if refMap, ok := ref.(map[string]interface{}); ok {
			apiVersion, _ := refMap["apiVersion"].(string)
			kind, _ := refMap["kind"].(string)
			name, _ := refMap["name"].(string)

			if apiVersion != "" && kind != "" && name != "" {
				// Parse apiVersion to get group and version
				parts := strings.Split(apiVersion, "/")
				group := ""
				version := ""
				if len(parts) == 2 {
					group = parts[0]
					version = parts[1]
				} else if len(parts) == 1 {
					version = parts[0]
				}

				key := fmt.Sprintf("%s/%s/%s/%s", group, version, kind, name)
				if _, exists := resourceMap[key]; !exists {
					resourceMap[key] = GenericResource{
						GVK: metav1.GroupVersionKind{
							Group:   group,
							Version: version,
							Kind:    kind,
						},
						Name:    name,
						Status:  resourceStatusPending,
						Message: "managed resource",
					}
				}
			}
		}
	}
}

// runTargetedDeletionLoop handles deletion of specific resources.
func (h *cleanupHelper) runTargetedDeletionLoop(cleanupCtx, ctx context.Context, cfg *cleanupConfig, resourceMap map[string]GenericResource, stage string) *CleanupResult {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	attempts := 0
	initialCount := len(resourceMap)
	// Create a copy of the map to track remaining resources
	remainingResources := make(map[string]GenericResource)
	for k, v := range resourceMap {
		remainingResources[k] = v
	}

	for {
		select {
		case <-cleanupCtx.Done():
			// Timeout reached - get detailed information about remaining resources
			return h.handleCleanupTimeout(ctx, cfg, resourceMap, remainingResources, attempts, stage)

		case <-ticker.C:
			attempts++

			// Try to delete remaining resources
			h.deleteRemainingResources(ctx, remainingResources)

			// Check if all resources are deleted
			if len(remainingResources) == 0 {
				return h.buildSuccessResult(cfg, resourceMap, initialCount, attempts, stage)
			}
		}
	}
}

// getResourceKey generates a unique key for a resource.
func (h *cleanupHelper) getResourceKey(r GenericResource) string {
	return fmt.Sprintf("%s/%s/%s/%s", r.GVK.Group, r.GVK.Version, r.GVK.Kind, r.Name)
}

// deleteRemainingResources attempts to delete all remaining resources.
func (h *cleanupHelper) deleteRemainingResources(ctx context.Context, remainingResources map[string]GenericResource) {
	for key, resource := range remainingResources {
		obj := h.createUnstructuredObject(resource)

		err := h.client.Delete(ctx, obj)
		if err != nil && kerrors.IsNotFound(err) {
			// Resource already deleted
			delete(remainingResources, key)
		}
	}
}

// createUnstructuredObject creates an unstructured object from a GenericResource.
func (h *cleanupHelper) createUnstructuredObject(r GenericResource) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   r.GVK.Group,
		Version: r.GVK.Version,
		Kind:    r.GVK.Kind,
	})
	obj.SetName(r.Name)
	if r.Namespace != "" {
		obj.SetNamespace(r.Namespace)
	}
	return obj
}

// handleCleanupTimeout processes resources when cleanup times out.
func (h *cleanupHelper) handleCleanupTimeout(ctx context.Context, cfg *cleanupConfig, originalResourceMap map[string]GenericResource, remainingResources map[string]GenericResource, attempts int, stage string) *CleanupResult {
	finalResources := make([]GenericResource, 0, len(originalResourceMap))
	deletedCount := 0

	for key, r := range originalResourceMap {
		if _, exists := remainingResources[key]; exists {
			// Resource still exists - fetch current state
			r = h.fetchResourceState(ctx, r)
			if r.Status == resourceStatusDeleted {
				deletedCount++
			}
		} else {
			r.Status = resourceStatusDeleted
			r.Message = "successfully deleted"
			deletedCount++
		}
		finalResources = append(finalResources, r)
	}

	result := &CleanupResult{
		DeletedCount:   deletedCount,
		RemainingCount: len(remainingResources),
		Resources:      finalResources,
		Attempts:       attempts,
		Errors:         []error{},
	}

	h.sendEventStatus(cfg, stage, len(remainingResources) == 0)
	return result
}

// fetchResourceState fetches the current state of a resource with detailed info.
func (h *cleanupHelper) fetchResourceState(ctx context.Context, r GenericResource) GenericResource {
	obj := h.createUnstructuredObject(r)

	err := h.client.Get(ctx, types.NamespacedName{
		Name:      r.Name,
		Namespace: r.Namespace,
	}, obj)
	if err != nil {
		if kerrors.IsNotFound(err) {
			r.Status = resourceStatusDeleted
			r.Message = "successfully deleted"
		} else {
			r.Status = resourceStatusFailed
			r.Message = fmt.Sprintf("deletion timed out: %v", err)
		}
		return r
	}

	// Resource still exists - get detailed info
	r.Status = resourceStatusFailed
	r.Message = "still exists"

	if obj.GetDeletionTimestamp() != nil {
		r.Status = resourceStatusPending
		r.Message = "pending deletion"
	}

	// Get synced condition message if available
	if syncedMessage := h.getSyncedConditionMessage(obj); syncedMessage != "" {
		r.Message = syncedMessage
	}

	// Get external name
	r.ExternalName = getExternalName(obj)

	return r
}

// buildSuccessResult builds the result when all resources are successfully deleted.
func (h *cleanupHelper) buildSuccessResult(cfg *cleanupConfig, resourceMap map[string]GenericResource, initialCount, attempts int, stage string) *CleanupResult {
	finalResources := make([]GenericResource, 0, len(resourceMap))
	for _, r := range resourceMap {
		r.Status = resourceStatusDeleted
		r.Message = "successfully deleted"
		finalResources = append(finalResources, r)
	}

	result := &CleanupResult{
		DeletedCount:   initialCount,
		RemainingCount: 0,
		Resources:      finalResources,
		Attempts:       attempts,
		Errors:         []error{},
	}

	h.sendEventStatus(cfg, stage, true)
	return result
}

// sendEventStatus sends event status if event channel is configured.
func (h *cleanupHelper) sendEventStatus(cfg *cleanupConfig, stage string, success bool) {
	if cfg.eventChannel != nil {
		if success {
			cfg.eventChannel.SendEvent(stage, async.EventStatusSuccess)
		} else {
			cfg.eventChannel.SendEvent(stage, async.EventStatusFailure)
		}
	}
}

// getSyncedConditionMessage extracts the message from the Synced condition.
func (h *cleanupHelper) getSyncedConditionMessage(item *unstructured.Unstructured) string {
	statusObj, ok := item.Object["status"].(map[string]interface{})
	if !ok {
		return ""
	}

	conditions, ok := statusObj["conditions"].([]interface{})
	if !ok {
		return ""
	}

	for _, cond := range conditions {
		condMap, ok := cond.(map[string]interface{})
		if !ok {
			continue
		}

		if condType, ok := condMap["type"].(string); ok && condType == "Synced" {
			if message, ok := condMap["message"].(string); ok {
				return message
			}
		}
	}

	return ""
}

// getExternalName extracts the external name annotation from a resource.
func getExternalName(item *unstructured.Unstructured) string {
	annotations := item.GetAnnotations()
	if annotations != nil {
		if externalName, ok := annotations["crossplane.io/external-name"]; ok {
			return externalName
		}
	}
	return ""
}

// getXPAPIResources discovers Crossplane API resources.
func getXPAPIResources(dc discovery.DiscoveryInterface) ([]metav1.GroupVersionKind, error) {
	apis, err := dc.ServerPreferredResources()
	if err != nil {
		// Discovery can return partial results with an error
		// Check if we have any results to work with.
		if apis == nil {
			return nil, errors.Wrap(err, "failed to get api resources")
		}
	}

	var resources []metav1.GroupVersionKind
	for _, api := range apis {
		gv, err := schema.ParseGroupVersion(api.GroupVersion)
		if err != nil {
			continue // Skip malformed group versions.
		}

		for _, resource := range api.APIResources {
			// Only process resources that can be listed and deleted.
			if !slices.Contains(resource.Verbs, "list") || !slices.Contains(resource.Verbs, "delete") {
				continue
			}

			cats := sets.New(resource.Categories...)
			if cats.HasAny("claim", "composite", "managed") {
				resources = append(resources, metav1.GroupVersionKind{
					Group:   gv.Group,
					Version: gv.Version,
					Kind:    resource.Kind,
				})
			}
		}
	}

	return resources, nil
}
