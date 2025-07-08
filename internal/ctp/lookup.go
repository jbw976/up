// Copyright 2025 Upbound Inc.
// All rights reserved

package ctp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	docker "github.com/docker/docker/client"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kind "sigs.k8s.io/kind/pkg/cluster"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	intctx "github.com/upbound/up/internal/ctx"
	"github.com/upbound/up/internal/upbound"
)

// FindDevControlPlane finds and returns an existing dev control plane with
// specific parameters. The bool return value will be false if the control plane
// is not found.
func FindDevControlPlane(ctx context.Context, upCtx *upbound.Context, opts ...FindDevControlPlaneOption) (DevControlPlane, bool, error) {
	cfg := &findDevControlPlaneConfig{}

	// Apply functional options
	for _, opt := range opts {
		opt(cfg)
	}

	// Determine whether to look for a spaces dev ctp or a local dev ctp, as
	// follows:
	//
	// 1. If local was explicitly requested, respect that.
	// 2. If the user's kubeconfig points to a Space, use Spaces by default.
	// 3. Otherwise, use local by default.

	if _, _, err := intctx.GetCurrentGroup(ctx, upCtx); err == nil && !cfg.forceLocal {
		return findSpacesDevControlPlane(ctx, upCtx, cfg)
	}

	return findLocalDevControlPlane(ctx, upCtx, cfg)
}

func findSpacesDevControlPlane(ctx context.Context, upCtx *upbound.Context, cfg *findDevControlPlaneConfig) (DevControlPlane, bool, error) {
	kubeconfig, err := intctx.GetSpacesKubeconfig(ctx, upCtx)
	if err != nil {
		return nil, false, errors.Wrap(err, "cannot get kubeconfig for current spaces context")
	}
	restConfig, err := kubeconfig.ClientConfig()
	if err != nil {
		return nil, false, errors.Wrap(err, "cannot get rest config for spaces client")
	}
	spaceClient, err := client.New(restConfig, client.Options{})
	if err != nil {
		return nil, false, errors.Wrap(err, "cannot construct spaces client")
	}

	group := cfg.spacesConfig.group
	if group == "" {
		ns, _, err := kubeconfig.Namespace()
		if err != nil {
			return nil, false, errors.Wrap(err, "cannot determine default group")
		}
		if ns == "" {
			ns = "default"
		}
		group = ns
	}
	nn := types.NamespacedName{Name: cfg.name, Namespace: group}
	var ctp spacesv1beta1.ControlPlane

	err = spaceClient.Get(ctx, nn, &ctp)
	switch {
	case err == nil:
		if !isDevControlPlane(&ctp) && !cfg.spacesConfig.allowProd {
			return nil, false, errNotDevControlPlane
		}

	case kerrors.IsNotFound(err):
		return nil, false, nil

	default:
		// Unexpected error.
		return nil, false, errors.Wrap(err, "failed to check for control plane existence")
	}

	// Get client for the control plane
	space, _, err := intctx.GetCurrentGroup(ctx, upCtx)
	if err != nil {
		return nil, false, err
	}

	ctpKubeconfig, err := space.BuildKubeconfig(nn)
	if err != nil {
		return nil, false, err
	}

	ctpRestConfig, err := ctpKubeconfig.ClientConfig()
	if err != nil {
		return nil, false, err
	}

	ctpClient, err := client.New(ctpRestConfig, client.Options{})
	if err != nil {
		return nil, false, err
	}

	// Create and return the spacesDevControlPlane
	return &spacesDevControlPlane{
		spaceClient: spaceClient,
		group:       group,
		name:        cfg.name,
		client:      ctpClient,
		kubeconfig:  ctpKubeconfig,
		breadcrumbs: fmt.Sprintf("%s/%s/%s", space.Breadcrumbs().String(), group, cfg.name),
	}, true, nil
}

func findLocalDevControlPlane(ctx context.Context, _ *upbound.Context, cfg *findDevControlPlaneConfig) (DevControlPlane, bool, error) {
	provider := kind.NewProvider()

	kubeconfigFile, err := os.CreateTemp("", "up-*.kubeconfig")
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to create temporary kubeconfig")
	}
	// We don't need the file handle.
	_ = kubeconfigFile.Close()
	// Clean up the file when we're done, but don't try too hard. If it fails
	// the temporary kubeconfig will be left behind.
	defer func() { _ = os.Remove(kubeconfigFile.Name()) }()

	// Find a kind cluster with the relevant name. If it doesn't exist, the
	// control plane doesn't exist.
	existing, err := provider.List()
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to list kind clusters")
	}
	if !slices.Contains(existing, cfg.name) {
		return nil, false, nil
	}

	// Get a kubeconfig and client for the kind cluster.
	kubeconfigStr, err := provider.KubeConfig(cfg.name, false)
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to get kubeconfig for kind cluster")
	}

	kubeconfig, err := clientcmd.NewClientConfigFromBytes([]byte(kubeconfigStr))
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to parse kubeconfig")
	}

	restConfig, err := kubeconfig.ClientConfig()
	if err != nil {
		return nil, false, errors.Wrap(err, "cannot get rest config")
	}

	cl, err := client.New(restConfig, client.Options{})
	if err != nil {
		return nil, false, errors.Wrap(err, "cannot construct control plane client")
	}

	// Try to find the sideloading registry. If it doesn't exist, we may have
	// failed to start it, or it may have been deleted out of band. That's fine.
	registryName := cfg.name + "-registry"
	cli, err := docker.NewClientWithOpts(docker.WithAPIVersionNegotiation(), docker.FromEnv)
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to create docker client")
	}
	cs, err := cli.ContainerList(ctx, container.ListOptions{
		Filters: filters.NewArgs(filters.KeyValuePair{Key: "name", Value: registryName}),
	})
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to list containers")
	}
	var cid string
	if len(cs) > 0 {
		cid = cs[0].ID
	}

	return &localDevControlPlane{
		name:                cfg.name,
		kubeconfig:          kubeconfig,
		client:              cl,
		registryDir:         filepath.Join(os.TempDir(), "up-local-registry", cfg.name),
		registryContainerID: cid,
		registryHostname:    cfg.name + "-registry:5000",
	}, true, nil
}

// FindDevControlPlaneOption defines functional options for finding control
// planes.
type FindDevControlPlaneOption func(*findDevControlPlaneConfig)

// findDevControlPlaneConfig sets configuration options for finding dev
// control planes.
type findDevControlPlaneConfig struct {
	name         string
	forceLocal   bool
	spacesConfig spacesConfig
}

// FindForceLocal forces a local control plane to be found even if Spaces is
// available.
func FindForceLocal(f bool) FindDevControlPlaneOption {
	return func(cfg *findDevControlPlaneConfig) {
		cfg.forceLocal = f
	}
}

// FindSkipDevCheck allows the use of a production control plane.
func FindSkipDevCheck(s bool) FindDevControlPlaneOption {
	return func(cfg *findDevControlPlaneConfig) {
		cfg.spacesConfig.allowProd = s
	}
}

// FindWithSpacesGroup sets the name of the spaces group in which to find the
// control plane.
func FindWithSpacesGroup(g string) FindDevControlPlaneOption {
	return func(cfg *findDevControlPlaneConfig) {
		cfg.spacesConfig.group = g
	}
}

// FindWithControlPlaneName sets the name of the control plane to find.
func FindWithControlPlaneName(n string) FindDevControlPlaneOption {
	return func(cfg *findDevControlPlaneConfig) {
		cfg.name = n
	}
}
