// Copyright 2025 Upbound Inc.
// All rights reserved

package ctp

import (
	"context"

	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MockDevControlPlane implements the DevControlPlane interface for testing.
type MockDevControlPlane struct {
	client     client.Client
	kubeconfig clientcmd.ClientConfig
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
