// Copyright 2021 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package ctp handles functions for ctp management
package ctp

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/scheme"

	commonv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	xpkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	xpkgv1beta1 "github.com/crossplane/crossplane/apis/pkg/v1beta1"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	ctxcmd "github.com/upbound/up/cmd/up/ctx"
	"github.com/upbound/up/internal/async"
	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/upbound"
)

const (
	// DevControlPlaneClass is used in project and test commands.
	DevControlPlaneClass = "small"
	// DevControlPlaneAnnotation is used in project and test commands.
	DevControlPlaneAnnotation = "upbound.io/development-control-plane"
)

// EnsureControlPlane creates clients and controlPlane.
func EnsureControlPlane(ctx context.Context, upCtx *upbound.Context, spaceClient client.Client, allowProd bool, action string, ch async.EventChannel, ctp spacesv1beta1.ControlPlane) (client.Client, clientcmd.ClientConfig, error) {
	nn := types.NamespacedName{
		Name:      ctp.Name,
		Namespace: ctp.Namespace,
	}

	switch action {
	case "create":
		err := spaceClient.Get(ctx, nn, &ctp)
		switch {
		case err == nil:
			// Make sure it's a dev control plane and not being deleted.
			if !isDevControlPlane(&ctp) && !allowProd {
				return nil, nil, errors.New("control plane exists but is not a development control plane; use --skip-control-plane-check to skip this check")
			}
			if ctp.DeletionTimestamp != nil {
				return nil, nil, errors.New("control plane exists but is being deleted - retry after it finishes deleting")
			}

		case kerrors.IsNotFound(err):
			// Create a control plane.
			if err := createControlPlane(ctx, spaceClient, ch, ctp); err != nil {
				return nil, nil, err
			}

		default:
			// Unexpected error.
			return nil, nil, errors.Wrap(err, "failed to check for control plane existence")
		}

		ctpClient, sClient, err := getControlPlaneClient(ctx, upCtx, nn)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to get client for development control plane")
		}

		return ctpClient, sClient, nil

	case "delete":
		// never delete prod control plane
		if !allowProd {
			// Fetch the control plane to delete
			err := spaceClient.Get(ctx, nn, &ctp)
			if err != nil {
				if kerrors.IsNotFound(err) {
					return nil, nil, errors.New("control plane does not exist, nothing to delete")
				}
				return nil, nil, errors.Wrap(err, "failed to fetch control plane for deletion")
			}

			// Delete the control plane
			if err := spaceClient.Delete(ctx, &ctp); err != nil {
				return nil, nil, errors.Wrap(err, "failed to delete control plane")
			}
		}
		return nil, nil, nil

	default:
		return nil, nil, errors.New("invalid action; valid actions are 'create' or 'delete'")
	}
}

func isDevControlPlane(ctp *spacesv1beta1.ControlPlane) bool {
	if ctp.Annotations != nil && ctp.Annotations[DevControlPlaneAnnotation] == "true" {
		return true
	}

	// We didn't used to annotate the control planes created by `up project
	// run`, and dev MCPs created via the console won't have the annotation, so
	// also check the control plane class. We assume any control plane with the
	// "small" class is a dev MCP.
	if ctp.Spec.Class == DevControlPlaneClass {
		return true
	}

	return false
}

// getControlPlaneConfig gets a REST config for a given control plane within
// the space.
//
// TODO(adamwg): Mostly copied from simulations; this should be factored out
// into our kube package.
func getControlPlaneClient(ctx context.Context, upCtx *upbound.Context, ctp types.NamespacedName) (client.Client, clientcmd.ClientConfig, error) {
	po := clientcmd.NewDefaultPathOptions()
	var err error

	conf, err := po.GetStartingConfig()
	if err != nil {
		return nil, nil, err
	}
	state, err := ctxcmd.DeriveState(ctx, upCtx, conf, kube.GetIngressHost)
	if err != nil {
		return nil, nil, err
	}

	var ok bool
	var space ctxcmd.Space

	if space, ok = state.(ctxcmd.Space); !ok {
		if group, ok := state.(*ctxcmd.Group); ok {
			space = group.Space
		} else if ctp, ok := state.(*ctxcmd.ControlPlane); ok {
			space = ctp.Group.Space
		} else {
			return nil, nil, errors.New("current kubeconfig is not pointed at a space cluster")
		}
	}

	spaceClient, err := space.BuildKubeconfig(ctp)
	if err != nil {
		return nil, nil, err
	}

	kubeconfig, err := spaceClient.ClientConfig()
	if err != nil {
		return nil, nil, err
	}

	ctpClient, err := client.New(kubeconfig, client.Options{})
	if err != nil {
		return nil, nil, err
	}

	ctpSchemeBuilders := []*scheme.Builder{
		xpkgv1.SchemeBuilder,
		xpkgv1beta1.SchemeBuilder,
	}
	for _, bld := range ctpSchemeBuilders {
		if err := bld.AddToScheme(ctpClient.Scheme()); err != nil {
			return nil, nil, err
		}
	}

	return ctpClient, spaceClient, nil
}

func createControlPlane(ctx context.Context, cl client.Client, ch async.EventChannel, ctp spacesv1beta1.ControlPlane) error {
	evText := "Creating development control plane"
	ch.SendEvent(evText, async.EventStatusStarted)
	if err := cl.Create(ctx, &ctp); err != nil {
		ch.SendEvent(evText, async.EventStatusFailure)
		return errors.Wrap(err, "failed to create control plane")
	}

	nn := types.NamespacedName{
		Name:      ctp.Name,
		Namespace: ctp.Namespace,
	}
	err := wait.PollUntilContextCancel(ctx, time.Second, true, func(ctx context.Context) (done bool, err error) {
		err = cl.Get(ctx, nn, &ctp)
		if err != nil {
			return false, err
		}

		cond := ctp.Status.GetCondition(commonv1.TypeReady)
		return cond.Status == corev1.ConditionTrue, nil
	})
	if err != nil {
		ch.SendEvent(evText, async.EventStatusFailure)
		return errors.Wrap(err, "waiting for control plane to be ready")
	}

	ch.SendEvent(evText, async.EventStatusSuccess)

	return nil
}
