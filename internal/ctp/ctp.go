// Copyright 2025 Upbound Inc.
// All rights reserved

// Package ctp manages control planes for inner-loop development purposes.
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

	commonv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	"github.com/upbound/up/internal/async"
	intctx "github.com/upbound/up/internal/ctx"
	"github.com/upbound/up/internal/upbound"
)

const (
	// devControlPlaneClass is used in project and test commands.
	devControlPlaneClass = "small"
	// devControlPlaneAnnotation is used in project and test commands.
	devControlPlaneAnnotation = "upbound.io/development-control-plane"
)

// errNotDevControlPlane is used in project and test commands.
var errNotDevControlPlane = errors.New("control plane exists but is not a development control plane")

// EnsureDevControlPlaneOption defines functional options for configuring control plane behavior.
type EnsureDevControlPlaneOption func(*ensureDevControlPlaneConfig)

// ensureDevControlPlaneConfig sets configuration options for creating dev control
// planes.
type ensureDevControlPlaneConfig struct {
	spacesConfig spacesConfig
	eventChan    async.EventChannel
}

// spacesConfig holds spaces-specific configuration options for creating dev
// control planes.
type spacesConfig struct {
	group       string
	name        string
	allowProd   bool
	class       string
	annotations map[string]string
	crossplane  *spacesv1beta1.CrossplaneSpec
}

// defaultCrossplaneSpec returns the default Crossplane configuration.
func defaultCrossplaneSpec() *spacesv1beta1.CrossplaneSpec {
	return &spacesv1beta1.CrossplaneSpec{
		AutoUpgradeSpec: &spacesv1beta1.CrossplaneAutoUpgradeSpec{
			// TODO(adamwg): For now, dev MCPs always use the rapid
			// channel because they require Crossplane features that are
			// only present in 1.18+. We can stop hard-coding this later
			// when other channels are upgraded.
			Channel: ptr.To(spacesv1beta1.CrossplaneUpgradeRapid),
		},
	}
}

// WithEventChannel specifies an event channel for progress events.
func WithEventChannel(ch async.EventChannel) EnsureDevControlPlaneOption {
	return func(cfg *ensureDevControlPlaneConfig) {
		cfg.eventChan = ch
	}
}

// SkipDevCheck allows the use of a production control plane.
func SkipDevCheck(s bool) EnsureDevControlPlaneOption {
	return func(cfg *ensureDevControlPlaneConfig) {
		cfg.spacesConfig.allowProd = s
	}
}

// WithCrossplaneSpec sets a specific Crossplane configuration.
func WithCrossplaneSpec(crossplane spacesv1beta1.CrossplaneSpec) EnsureDevControlPlaneOption {
	return func(cfg *ensureDevControlPlaneConfig) {
		cfg.spacesConfig.crossplane = &crossplane
	}
}

// WithSpacesGroup sets the name of the spaces group in which to create the
// control plane.
func WithSpacesGroup(g string) EnsureDevControlPlaneOption {
	return func(cfg *ensureDevControlPlaneConfig) {
		cfg.spacesConfig.group = g
	}
}

// WithSpacesControlPlaneName sets the name of the spaces control plane to
// create.
func WithSpacesControlPlaneName(n string) EnsureDevControlPlaneOption {
	return func(cfg *ensureDevControlPlaneConfig) {
		cfg.spacesConfig.name = n
	}
}

// DevControlPlane is a control plane used for local development. It may run in
// a variety of ways.
type DevControlPlane interface {
	// Client returns a controller-runtime client for the control plane.
	Client() client.Client
	// Kubeconfig returns a kubeconfig for the control plane.
	Kubeconfig() clientcmd.ClientConfig
	// Teardown tears down the control plane, deleting any resources it may use.
	Teardown(ctx context.Context, force bool) error
}

// spacesDevControlPlane is a development control plane that runs in a Spaces
// cluster.
type spacesDevControlPlane struct {
	spaceClient client.Client
	group       string
	name        string

	client     client.Client
	kubeconfig clientcmd.ClientConfig
}

// Client returns a controller-runtime client for the control plane.
func (s *spacesDevControlPlane) Client() client.Client {
	return s.client
}

// Kubeconfig returns a kubeconfig for the control plane.
func (s *spacesDevControlPlane) Kubeconfig() clientcmd.ClientConfig {
	return s.kubeconfig
}

// Teardown tears down the control plane, deleting any resources it may use.
func (s *spacesDevControlPlane) Teardown(ctx context.Context, force bool) error {
	nn := types.NamespacedName{Name: s.name, Namespace: s.group}
	var ctp spacesv1beta1.ControlPlane

	// Fetch the control plane to delete
	err := s.spaceClient.Get(ctx, nn, &ctp)
	if err != nil {
		if kerrors.IsNotFound(err) {
			return errors.New("control plane does not exist, nothing to delete")
		}
		return errors.Wrap(err, "failed to fetch control plane for deletion")
	}

	// Never delete a production control plane unless force is set
	if !force && !isDevControlPlane(&ctp) {
		return errors.New("control plane exists but is not a development control plane")
	}

	// Delete the control plane
	if err := s.spaceClient.Delete(ctx, &ctp); err != nil {
		return errors.Wrap(err, "failed to delete control plane")
	}

	return nil
}

// EnsureDevControlPlane ensures the existence of a control plane for
// development.
func EnsureDevControlPlane(ctx context.Context, upCtx *upbound.Context, opts ...EnsureDevControlPlaneOption) (DevControlPlane, error) {
	cfg := &ensureDevControlPlaneConfig{
		spacesConfig: spacesConfig{
			class: devControlPlaneClass,
			annotations: map[string]string{
				devControlPlaneAnnotation: "true",
			},
			crossplane: defaultCrossplaneSpec(),
		},
	}

	// Apply functional options
	for _, opt := range opts {
		opt(cfg)
	}

	nn := types.NamespacedName{Name: cfg.spacesConfig.name, Namespace: cfg.spacesConfig.group}
	var ctp spacesv1beta1.ControlPlane

	kubeconfig, err := intctx.GetSpacesKubeconfig(ctx, upCtx)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get kubeconfig for current spaces context")
	}
	restConfig, err := kubeconfig.ClientConfig()
	if err != nil {
		return nil, errors.Wrap(err, "cannot get rest config for spaces client")
	}
	spaceClient, err := client.New(restConfig, client.Options{})
	if err != nil {
		return nil, errors.Wrap(err, "cannot construct spaces client")
	}

	err = spaceClient.Get(ctx, nn, &ctp)
	switch {
	case err == nil:
		// Make sure it's a dev control plane and not being deleted.
		if !isDevControlPlane(&ctp) && !cfg.spacesConfig.allowProd {
			return nil, errNotDevControlPlane
		}
		if ctp.DeletionTimestamp != nil {
			return nil, errors.New("control plane exists but is being deleted - retry after it finishes deleting")
		}
		// Ensure the Crossplane spec fully matches what the caller specified
		if !matchesCrossplaneSpec(ctp.Spec.Crossplane, *cfg.spacesConfig.crossplane) {
			return nil, errors.Errorf(
				"existing control plane has a different Crossplane spec than expected",
			)
		}

	case kerrors.IsNotFound(err):
		// Create a control plane.
		ctp = spacesv1beta1.ControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:        cfg.spacesConfig.name,
				Namespace:   cfg.spacesConfig.group,
				Annotations: cfg.spacesConfig.annotations,
			},
			Spec: spacesv1beta1.ControlPlaneSpec{
				Class:      cfg.spacesConfig.class,
				Crossplane: *cfg.spacesConfig.crossplane,
			},
		}

		if err := createControlPlane(ctx, spaceClient, cfg.eventChan, ctp); err != nil {
			return nil, err
		}

	default:
		// Unexpected error.
		return nil, errors.Wrap(err, "failed to check for control plane existence")
	}
	// Create a new control plane with the user-defined or default Crossplane configuration

	// Get client for the control plane
	ctpClient, sClient, err := getControlPlaneClient(ctx, upCtx, nn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get client for control plane")
	}

	// Create and return the spacesDevControlPlane
	return &spacesDevControlPlane{
		spaceClient: spaceClient,
		group:       cfg.spacesConfig.group,
		name:        cfg.spacesConfig.name,
		client:      ctpClient,
		kubeconfig:  sClient,
	}, nil
}

func isDevControlPlane(ctp *spacesv1beta1.ControlPlane) bool {
	if ctp.Annotations != nil && ctp.Annotations[devControlPlaneAnnotation] == "true" {
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
	space, _, err := intctx.GetCurrentGroup(ctx, upCtx)
	if err != nil {
		return nil, nil, err
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

	return ctpClient, spaceClient, nil
}

func createControlPlane(ctx context.Context, cl client.Client, ch async.EventChannel, ctp spacesv1beta1.ControlPlane) error {
	evText := "Creating development control plane"
	ch.SendEvent(evText, async.EventStatusStarted)

	backoff := wait.Backoff{
		Duration: 500 * time.Millisecond,
		Factor:   2.0,
		Jitter:   0.1,
		Steps:    6,
	}

	if err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		// Try to create the resource
		if err := cl.Create(ctx, &ctp); err != nil {
			// Check if it exists and is being deleted
			existing := &spacesv1beta1.ControlPlane{}
			getErr := cl.Get(ctx, client.ObjectKey{
				Name:      ctp.Name,
				Namespace: ctp.Namespace,
			}, existing)

			if getErr == nil && existing.DeletionTimestamp != nil {
				// It's being deleted, so retry
				return false, nil
			}
			// Not a retryable error
			return false, err
		}

		// Successfully created
		return true, nil
	}); err != nil {
		ch.SendEvent(evText, async.EventStatusFailure)
		return errors.Wrap(err, "failed to create control plane")
	}

	nn := types.NamespacedName{
		Name:      ctp.Name,
		Namespace: ctp.Namespace,
	}
	if err := wait.PollUntilContextCancel(ctx, time.Second, true, func(ctx context.Context) (done bool, err error) {
		err = cl.Get(ctx, nn, &ctp)
		if err != nil {
			return false, err
		}

		cond := ctp.Status.GetCondition(commonv1.TypeReady)
		return cond.Status == corev1.ConditionTrue, nil
	}); err != nil {
		ch.SendEvent(evText, async.EventStatusFailure)
		return errors.Wrap(err, "waiting for control plane to be ready")
	}

	ch.SendEvent(evText, async.EventStatusSuccess)

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
