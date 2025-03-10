// Copyright 2025 Upbound Inc.
// All rights reserved

package importer

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/upbound/up/pkg/migration/category"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ResourceImporter interface {
	ImportResources(ctx context.Context, gr string, restoreStatus bool) (int, error)
}

type PausingResourceImporter struct {
	reader  ResourceReader
	applier ResourceApplier
}

func NewPausingResourceImporter(r ResourceReader, a ResourceApplier) *PausingResourceImporter {
	return &PausingResourceImporter{
		reader:  r,
		applier: a,
	}
}

func (im *PausingResourceImporter) ImportResources(ctx context.Context, gr string, restoreStatus, pausedBeforeExport bool, mcpConnectorClusterID, mcpConnectorClaimNamespace string) (int, error) {
	resources, typeMeta, err := im.reader.ReadResources(gr)
	if err != nil {
		return 0, errors.Wrapf(err, "cannot get %q resources", gr)
	}

	hasSubresource := false

	if typeMeta != nil && mcpConnectorClaimNamespace != "" && mcpConnectorClusterID != "" {

		for _, c := range typeMeta.Categories {

			if mcpConnectorClaimNamespace != "default" {
				// Create Namespace resource dynamically for mcpConnectorClaimNamespace
				namespaceResource := &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "Namespace",
						"metadata": map[string]interface{}{
							"name": mcpConnectorClaimNamespace,
						},
					},
				}

				if err = im.applier.ApplyResources(ctx, []unstructured.Unstructured{*namespaceResource}, false); err != nil {
					return 0, errors.Wrapf(err, "cannot apply %q namespace", gr)
				}
			}

			if c == "claim" {
				for i := range resources {
					namespace, _, _ := unstructured.NestedString(resources[i].Object, "metadata", "namespace")
					name, _, _ := unstructured.NestedString(resources[i].Object, "metadata", "name")

					// Set labels correctly
					meta.AddLabels(&resources[i], map[string]string{
						"mcp-connector.upbound.io/app-namespace":     namespace,
						"mcp-connector.upbound.io/app-resource-name": name,
						"mcp-connector.upbound.io/app-cluster":       mcpConnectorClusterID,
					})

					// Compute hash for name
					// https://github.com/upbound/mcp-connector/blob/1bebcf281d22bd2ec6d5ddbe8184e26cdc193a90/pkg/rest/client/translator/translator.go#L70
					h := sha256.New()
					_, _ = h.Write([]byte(name + "-x-" + namespace + "-x-" + mcpConnectorClusterID))
					hash := fmt.Sprintf("%x", h.Sum(nil))
					// Use the first 16 characters of the hash for the new name
					newName := fmt.Sprintf("claim-%s", hash[:16])

					// Update the metadata fields properly
					_ = unstructured.SetNestedField(resources[i].Object, newName, "metadata", "name")
					_ = unstructured.SetNestedField(resources[i].Object, mcpConnectorClaimNamespace, "metadata", "namespace")
				}
			}

			if c == "composite" {
				for i := range resources {
					// Extract claimRef fields correctly
					claimRef, found, _ := unstructured.NestedMap(resources[i].Object, "spec", "claimRef")
					if found {
						claimName, _, _ := unstructured.NestedString(claimRef, "name")
						claimNamespace, _, _ := unstructured.NestedString(claimRef, "namespace")

						// Compute hash for name
						h := sha256.New()
						_, _ = h.Write([]byte(claimName + "-x-" + claimNamespace + "-x-" + mcpConnectorClusterID))
						hash := fmt.Sprintf("%x", h.Sum(nil))

						// Use the first 16 characters of the hash for the new name
						newName := fmt.Sprintf("claim-%s", hash[:16])

						// Set the new values in claimRef
						_ = unstructured.SetNestedField(claimRef, newName, "name")
						_ = unstructured.SetNestedField(claimRef, mcpConnectorClaimNamespace, "namespace")

						// Update the resource with modified claimRef
						_ = unstructured.SetNestedField(resources[i].Object, claimRef, "spec", "claimRef")
					}
				}
			}
		}
	}

	// We pause all resources that are managed, claim, or composite, if they not paused before in export
	if !pausedBeforeExport && typeMeta != nil {
		hasSubresource = typeMeta.WithStatusSubresource
		for _, c := range typeMeta.Categories {
			// - Claim/Composite: We don't want Crossplane controllers to create new resources before we import all.
			// - Managed: Same reason as above, but also don't want to take control of cloud resources yet.
			if c == "managed" || c == "claim" || c == "composite" {
				for i := range resources {
					annotations := resources[i].GetAnnotations()
					if annotations["crossplane.io/paused"] == "true" {
						// If already paused, add the migration-specific annotation
						meta.AddAnnotations(&resources[i], map[string]string{
							"migration.upbound.io/already-paused": "true",
						})
					} else {
						// Otherwise, add the crossplane pause annotation
						meta.AddAnnotations(&resources[i], map[string]string{
							"crossplane.io/paused": "true",
						})
					}
				}
				break
			}
		}
	}

	if err = im.applier.ApplyResources(ctx, resources, restoreStatus && hasSubresource); err != nil {
		return 0, errors.Wrapf(err, "cannot apply %q resources", gr)
	}

	return len(resources), nil
}

func (im *PausingResourceImporter) UnpauseResources(ctx context.Context, resourceType string, cm *category.APICategoryModifier) error {
	_, err := cm.ModifyResources(ctx, resourceType, func(u *unstructured.Unstructured) error {
		annotations := u.GetAnnotations()
		alreadyPaused, exists := annotations["migration.upbound.io/already-paused"]
		if !exists || alreadyPaused == "false" {
			meta.RemoveAnnotations(u, "crossplane.io/paused")
		}
		return nil
	})
	return err
}
