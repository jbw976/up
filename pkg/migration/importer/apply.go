// Copyright 2025 Upbound Inc.
// All rights reserved

package importer

import (
	"context"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	for i := range resources {
		err := retry.OnError(retry.DefaultRetry, resource.IsAPIError, func() error {
			rm, err := a.resourceMapper.RESTMapping(resources[i].GroupVersionKind().GroupKind(), resources[i].GroupVersionKind().Version)
			if err != nil {
				return err
			}

			rs := resources[i].DeepCopy()
			_, err = a.dynamicClient.Resource(rm.Resource).Namespace(resources[i].GetNamespace()).Apply(ctx, resources[i].GetName(), &resources[i], v1.ApplyOptions{
				FieldManager: "up-controlplane-migrator",
				Force:        true,
			})
			if err != nil {
				return err
			}
			if !applyStatus {
				return nil
			}
			_, err = a.dynamicClient.Resource(rm.Resource).Namespace(resources[i].GetNamespace()).ApplyStatus(ctx, rs.GetName(), rs, v1.ApplyOptions{
				FieldManager: "up-controlplane-migrator",
				Force:        true,
			})
			if err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return errors.Wrapf(err, "cannot apply resource %s/%s", resources[i].GetKind(), resources[i].GetName())
		}
	}
	return nil
}

func (a *UnstructuredResourceApplier) ModifyResources(ctx context.Context, resources []unstructured.Unstructured, modify func(*unstructured.Unstructured) error) error {
	for i := range resources {
		err := retry.OnError(retry.DefaultRetry, resource.IsAPIError, func() error {
			rm, err := a.resourceMapper.RESTMapping(resources[i].GroupVersionKind().GroupKind(), resources[i].GroupVersionKind().Version)
			if err != nil {
				return err
			}
			u, err := a.dynamicClient.Resource(rm.Resource).Namespace(resources[i].GetNamespace()).Get(ctx, resources[i].GetName(), v1.GetOptions{})
			if err != nil {
				return err
			}
			if err := modify(u); err != nil {
				return err
			}

			_, err = a.dynamicClient.Resource(rm.Resource).Namespace(resources[i].GetNamespace()).Update(ctx, u, v1.UpdateOptions{})
			if err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return errors.Wrapf(err, "cannot apply resource %s/%s", resources[i].GetKind(), resources[i].GetName())
		}
	}
	return nil
}
