// Copyright 2025 Upbound Inc.
// All rights reserved

package importer

import (
	"context"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/util/retry"
)

type ResourceApplier interface {
	ApplyResources(ctx context.Context, resources []unstructured.Unstructured, applyStatus bool) error
	ModifyResources(ctx context.Context, resources []unstructured.Unstructured, modify func(*unstructured.Unstructured) error) error
}

type UnstructuredResourceApplier struct {
	dynamicClient  dynamic.Interface
	resourceMapper meta.RESTMapper
}

func NewUnstructuredResourceApplier(dynamicClient dynamic.Interface, resourceMapper meta.RESTMapper) *UnstructuredResourceApplier {
	return &UnstructuredResourceApplier{
		dynamicClient:  dynamicClient,
		resourceMapper: resourceMapper,
	}
}

func (a *UnstructuredResourceApplier) ApplyResources(ctx context.Context, resources []unstructured.Unstructured, applyStatus bool) error {
	// It's possible that webhook if providers are not ready - and the import will fail.
	for i := range resources {
		err := wait.ExponentialBackoff(wait.Backoff{
			Duration: 500 * time.Millisecond, // Initial retry delay
			Factor:   2.0,                    // Doubles each time
			Jitter:   0.1,                    // 10% random jitter
			Steps:    5,                      // Maximum retries
		}, func() (bool, error) {
			rm, err := a.resourceMapper.RESTMapping(resources[i].GroupVersionKind().GroupKind(), resources[i].GroupVersionKind().Version)
			if err != nil {
				return false, errors.Wrap(err, "cannot get REST mapping") // Retryable error
			}

			rs := resources[i].DeepCopy()

			_, err = a.dynamicClient.Resource(rm.Resource).Namespace(resources[i].GetNamespace()).Apply(ctx, resources[i].GetName(), &resources[i], v1.ApplyOptions{
				FieldManager: "up-controlplane-migrator",
				Force:        true,
			})
			if err != nil {
				if resource.IsAPIErrorWrapped(err) {
					return false, nil // Retry
				}
				return false, errors.Wrap(err, "cannot apply resource") // Non-retryable error
			}

			if !applyStatus {
				return true, nil // Success, no need for status update
			}

			_, err = a.dynamicClient.Resource(rm.Resource).Namespace(resources[i].GetNamespace()).ApplyStatus(ctx, rs.GetName(), rs, v1.ApplyOptions{
				FieldManager: "up-controlplane-migrator",
				Force:        true,
			})
			if err != nil {
				// Note: We just successfully applied this resource above,
				// so we can ignore the not found error here. This could happen if
				// the resource was being deleted during export and garbage
				// collected right after the resource was applied. In this case,
				// we will get a not found error here, while applying the status,
				// so we can ignore it.
				if resource.IgnoreNotFound(err) != nil {
					if resource.IsAPIError(err) {
						return false, nil // Retry
					}
					return false, errors.Wrap(err, "cannot apply resource status") // Non-retryable error
				}
			}

			return true, nil // Successfully applied both resource and status
		})

		if err != nil {
			return errors.Wrapf(err, "cannot apply resource %s/%s", resources[i].GetKind(), resources[i].GetName())
		}
	}
	return nil
}

func (a *UnstructuredResourceApplier) ModifyResources(ctx context.Context, resources []unstructured.Unstructured, modify func(*unstructured.Unstructured) error) error {
	for i := range resources {
		err := retry.OnError(retry.DefaultRetry, resource.IsAPIErrorWrapped, func() error {
			rm, err := a.resourceMapper.RESTMapping(resources[i].GroupVersionKind().GroupKind(), resources[i].GroupVersionKind().Version)
			if err != nil {
				return errors.Wrap(err, "cannot get REST mapping")
			}
			u, err := a.dynamicClient.Resource(rm.Resource).Namespace(resources[i].GetNamespace()).Get(ctx, resources[i].GetName(), v1.GetOptions{})
			if err != nil {
				return errors.Wrap(err, "cannot get resource")
			}
			if err := modify(u); err != nil {
				return errors.Wrap(err, "cannot modify resource")
			}

			_, err = a.dynamicClient.Resource(rm.Resource).Namespace(resources[i].GetNamespace()).Update(ctx, u, v1.UpdateOptions{})
			if err != nil {
				return errors.Wrap(err, "cannot update resource")
			}
			return nil
		})
		if err != nil {
			return errors.Wrapf(err, "cannot apply resource %s/%s", resources[i].GetKind(), resources[i].GetName())
		}
	}
	return nil
}
