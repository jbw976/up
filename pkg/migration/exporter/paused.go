// Copyright 2025 Upbound Inc.
// All rights reserved

package exporter

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	xpmeta "github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/upbound/up/pkg/migration"
	"github.com/upbound/up/pkg/migration/category"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
)

type ResourcePauser interface {
	PauseResources(ctx context.Context, categoryName string) error
}

type DefaultResourcePauser struct {
	dynamicClient   dynamic.Interface
	discoveryClient discovery.DiscoveryInterface
}

func NewDefaultResourcePauser(dynamicClient dynamic.Interface, discoveryClient discovery.DiscoveryInterface) *DefaultResourcePauser {
	return &DefaultResourcePauser{
		dynamicClient:   dynamicClient,
		discoveryClient: discoveryClient,
	}
}

func (rp *DefaultResourcePauser) PauseResources(ctx context.Context, categoryName string) error {
	pauseMsg := fmt.Sprintf("Pausing all %s resources before export... ", categoryName)
	s, _ := migration.DefaultSpinner.Start(pauseMsg)
	cm := category.NewAPICategoryModifier(rp.dynamicClient, rp.discoveryClient)

	// Modify all resources of the specified category to add the "crossplane.io/paused: true" annotation.
	// Additionally, check if a resource is already paused by verifying the "crossplane.io/paused" annotation.
	// If the resource was previously paused, add the "migration.upbound.io/already-paused: true" annotation.
	count, err := cm.ModifyResources(ctx, categoryName, func(u *unstructured.Unstructured) error {
		annotations := u.GetAnnotations()
		if isPaused, exists := annotations["crossplane.io/paused"]; exists && isPaused == "true" {
			// Add the migration annotation only if already paused
			xpmeta.AddAnnotations(u, map[string]string{
				"migration.upbound.io/already-paused": "true",
			})
		} else {
			xpmeta.AddAnnotations(u, map[string]string{
				"crossplane.io/paused": "true",
			})
		}
		return nil
	})
	if err != nil {
		s.Fail(pauseMsg + stepFailed)
		return errors.Wrapf(err, "cannot pause %s resources", categoryName)
	}
	s.Success(pauseMsg + fmt.Sprintf("%d resources paused! ⏸️", count))

	return nil
}
