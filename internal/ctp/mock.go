// Copyright 2025 Upbound Inc.
// All rights reserved

package ctp

import (
	"context"

	"github.com/google/go-containerregistry/pkg/name"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/upbound/up/internal/project"
)

// MockDevControlPlane implements the DevControlPlane interface for testing.
type MockDevControlPlane struct {
	client        client.Client
	kubeconfig    clientcmd.ClientConfig
	cleanupResult *CleanupResult
	cleanupErr    error
}

// NewMockDevControlPlane creates a new MockDevControlPlane with the provided client and kubeconfig.
func NewMockDevControlPlane(client client.Client, kubeconfig clientcmd.ClientConfig) *MockDevControlPlane {
	return &MockDevControlPlane{
		client:     client,
		kubeconfig: kubeconfig,
	}
}

// Info returns human-friendly information about the mock control plane.
func (m *MockDevControlPlane) Info() string {
	return "Mock development control plane"
}

// ShortDescription returns a short description.
func (m *MockDevControlPlane) ShortDescription() string {
	return "Mock development control plane"
}

// Client returns the controller-runtime client for the mock control plane.
func (m *MockDevControlPlane) Client() client.Client {
	return m.client
}

// Kubeconfig returns the kubeconfig for the mock control plane.
func (m *MockDevControlPlane) Kubeconfig() clientcmd.ClientConfig {
	return m.kubeconfig
}

// Teardown is a no-op implementation for the mock control plane.
func (m *MockDevControlPlane) Teardown(_ context.Context, _ bool) error {
	return nil
}

// MockSideloadingDevControlPlane implements the SideloadingDevControlPlane
// interface for testing. The Sideload method calls the SideloadFn callback.
type MockSideloadingDevControlPlane struct {
	MockDevControlPlane
	SideloadFn func(ctx context.Context, imgMap project.ImageTagMap, tag name.Tag) error
}

// Sideload calls the SideloadFn callback.
func (m *MockSideloadingDevControlPlane) Sideload(ctx context.Context, imgMap project.ImageTagMap, tag name.Tag) error {
	return m.SideloadFn(ctx, imgMap, tag)
}

// Cleanup is a no-op implementation for the mock control plane.
func (m *MockDevControlPlane) Cleanup(_ context.Context, _ ...CleanupOption) (*CleanupResult, error) {
	if m.cleanupErr != nil {
		return nil, m.cleanupErr
	}

	if m.cleanupResult != nil {
		return m.cleanupResult, nil
	}

	return &CleanupResult{
		DeletedCount:   0,
		RemainingCount: 0,
		Resources:      []GenericResource{},
		Errors:         []error{},
		Attempts:       1,
	}, nil
}

// WithCleanupResult sets a specific result to be returned by Cleanup.
func (m *MockDevControlPlane) WithCleanupResult(result *CleanupResult) *MockDevControlPlane {
	m.cleanupResult = result
	return m
}

// WithCleanupError sets an error to be returned by Cleanup.
func (m *MockDevControlPlane) WithCleanupError(err error) *MockDevControlPlane {
	m.cleanupErr = err
	return m
}
