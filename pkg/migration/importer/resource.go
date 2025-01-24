// Copyright 2025 Upbound Inc.
// All rights reserved

package importer

import (
	"context"

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

func (im *PausingResourceImporter) ImportResources(ctx context.Context, gr string, restoreStatus bool, pausedBeforeExport bool) (int, error) {
	resources, typeMeta, err := im.reader.ReadResources(gr)
	if err != nil {
		return 0, errors.Wrapf(err, "cannot get %q resources", gr)
	}

	hasSubresource := false
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
