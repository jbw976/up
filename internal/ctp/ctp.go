// Copyright 2025 Upbound Inc.
// All rights reserved

// Package ctp handles functions for ctp management
package ctp

import (
	"context"
	"time"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"
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

// ErrNotDevControlPlane is used in project and test commands.
var ErrNotDevControlPlane = errors.New("control plane exists but is not a development control plane")

// EnsureControlPlaneOption defines functional options for configuring control plane behavior.
type EnsureControlPlaneOption func(*ensureControlPlaneConfig)

type ensureControlPlaneConfig struct {
	allowProd   bool
	class       string
	annotations map[string]string
	crossplane  *spacesv1beta1.CrossplaneSpec
}

// defaultCrossplaneSpec returns the default Crossplane configuration.
func defaultCrossplaneSpec() spacesv1beta1.CrossplaneSpec {
	return spacesv1beta1.CrossplaneSpec{
		AutoUpgradeSpec: &spacesv1beta1.CrossplaneAutoUpgradeSpec{
			// TODO(adamwg): For now, dev MCPs always use the rapid
			// channel because they require Crossplane features that are
			// only present in 1.18+. We can stop hard-coding this later
			// when other channels are upgraded.
			Channel: ptr.To(spacesv1beta1.CrossplaneUpgradeRapid),
		},
	}
}

// DevControlPlane sets the control plane to be a development environment.
func DevControlPlane() EnsureControlPlaneOption {
	return func(cfg *ensureControlPlaneConfig) {
		cfg.class = DevControlPlaneClass
		cfg.annotations = map[string]string{
			DevControlPlaneAnnotation: "true",
		}
	}
}

// SkipDevCheck allows the use of a production control plane.
func SkipDevCheck(s bool) EnsureControlPlaneOption {
	return func(cfg *ensureControlPlaneConfig) {
		cfg.allowProd = s
	}
}

// WithCrossplaneSpec sets a specific Crossplane configuration.
func WithCrossplaneSpec(crossplane spacesv1beta1.CrossplaneSpec) EnsureControlPlaneOption {
	return func(cfg *ensureControlPlaneConfig) {
		cfg.crossplane = &crossplane
	}
}

// EnsureControlPlane ensures the existence of a control plane.
func EnsureControlPlane(ctx context.Context, upCtx *upbound.Context, spaceClient client.Client, group, name string, ch async.EventChannel, opts ...EnsureControlPlaneOption) (client.Client, clientcmd.ClientConfig, error) {
	cfg := &ensureControlPlaneConfig{}

	// Apply functional options
	for _, opt := range opts {
		opt(cfg)
	}

	if cfg.crossplane == nil {
		defaultSpec := defaultCrossplaneSpec()
		cfg.crossplane = &defaultSpec
	}

	nn := types.NamespacedName{Name: name, Namespace: group}
	var ctp spacesv1beta1.ControlPlane

	err := spaceClient.Get(ctx, nn, &ctp)
	switch {
	case err == nil:
		// Make sure it's a dev control plane and not being deleted.
		if !isDevControlPlane(&ctp) && !cfg.allowProd {
			return nil, nil, ErrNotDevControlPlane
		}
		if ctp.DeletionTimestamp != nil {
			return nil, nil, errors.New("control plane exists but is being deleted - retry after it finishes deleting")
		}
		// Ensure the Crossplane spec fully matches what the caller specified
		if !matchesCrossplaneSpec(ctp.Spec.Crossplane, *cfg.crossplane) {
			return nil, nil, errors.Errorf(
				"existing control plane has a different Crossplane spec than expected",
			)
		}

	case kerrors.IsNotFound(err):
		// Create a control plane.
		ctp = spacesv1beta1.ControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:        name,
				Namespace:   group,
				Annotations: cfg.annotations,
			},
			Spec: spacesv1beta1.ControlPlaneSpec{
				Class:      cfg.class,
				Crossplane: *cfg.crossplane,
			},
		}

		if err := createControlPlane(ctx, spaceClient, ch, ctp); err != nil {
			return nil, nil, err
		}

	default:
		// Unexpected error.
		return nil, nil, errors.Wrap(err, "failed to check for control plane existence")
	}
	// Create a new control plane with the user-defined or default Crossplane configuration

	// Get client for the control plane
	ctpClient, sClient, err := getControlPlaneClient(ctx, upCtx, nn)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get client for control plane")
	}

	return ctpClient, sClient, nil
}

func isDevControlPlane(ctp *spacesv1beta1.ControlPlane) bool {
	if ctp.Annotations != nil && ctp.Annotations[DevControlPlaneAnnotation] == "true" {
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

// DeleteControlPlane deletes a control plane, ensuring production control planes are not deleted accidentally.
func DeleteControlPlane(ctx context.Context, spaceClient client.Client, group, name string, allowProd bool) error {
	nn := types.NamespacedName{Name: name, Namespace: group}
	var ctp spacesv1beta1.ControlPlane

	// Fetch the control plane to delete
	err := spaceClient.Get(ctx, nn, &ctp)
	if err != nil {
		if kerrors.IsNotFound(err) {
			return errors.New("control plane does not exist, nothing to delete")
		}
		return errors.Wrap(err, "failed to fetch control plane for deletion")
	}

	// Never delete a production control plane unless allowProd is set
	if !allowProd && !isDevControlPlane(&ctp) {
		return errors.New("control plane does not exist, nothing to delete")
	}

	// Delete the control plane
	if err := spaceClient.Delete(ctx, &ctp); err != nil {
		return errors.Wrap(err, "failed to delete control plane")
	}

	return nil
}

func matchesCrossplaneSpec(existing, desired spacesv1beta1.CrossplaneSpec) bool {
	// Spaces applies defaults to the CrossplaneSpec, so we can't compare the
	// full structs. Ignore the version and state unless they're set in our
	// desired spec.

	if desired.Version == nil {
		existing.Version = nil
	}
	if desired.State == nil {
		existing.State = nil
	}

	return cmp.Equal(existing, desired)
}
