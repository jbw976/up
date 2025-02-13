// Copyright 2025 Upbound Inc.
// All rights reserved

package kube

import (
	"context"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

const errClosedResults = "stopped watching before condition met"

// DynamicWatch starts a watch on the given resource type. The done callback is
// called on every received event until either timeout or context cancellation.
func DynamicWatch(ctx context.Context, r dynamic.NamespaceableResourceInterface, timeout *int64, done func(u *unstructured.Unstructured) (bool, error)) (chan error, error) {
	w, err := r.Watch(ctx, v1.ListOptions{
		TimeoutSeconds: timeout,
	})
	if err != nil {
		return nil, err
	}
	errChan := make(chan error)
	go func() {
		defer close(errChan)
		for {
			select {
			case e, ok := <-w.ResultChan():
				// If we are no longer watching return with error.
				if !ok {
					errChan <- errors.New(errClosedResults)
					return
				}

				u, ok := e.Object.(*unstructured.Unstructured)
				if !ok {
					continue
				}

				// If we error on event callback return early.
				d, err := done(u)
				if err != nil {
					errChan <- err
					return
				}
				// If event callback indicated done, return early with nil
				// error.
				if d {
					errChan <- nil
					return
				}
			// If context is canceled, stop watching and return error.
			case <-ctx.Done():
				w.Stop()
				errChan <- ctx.Err()
				return
			}
		}
	}()
	return errChan, nil
}
