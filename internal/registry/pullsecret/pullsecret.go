// Copyright 2025 Upbound Inc.
// All rights reserved

// Package pullsecret contains types for OCI registry pull secrets.
package pullsecret

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/registry"
)

const (
	errFmtCreateNamespace = "failed to create pull secret namespace %q"
)

// Manager manages a registry pull secret.
type Manager struct {
	name       string
	namespace  string
	username   string
	password   string
	endpoint   string
	client     kubernetes.Interface
	pullSecret *kube.ImagePullApplicator
}

// NewManager returns an initialized *Manager.
func NewManager(client kubernetes.Interface, name, namespace, username, password, endpoint string) *Manager {
	return &Manager{
		name:       name,
		namespace:  namespace,
		username:   username,
		password:   password,
		endpoint:   endpoint,
		client:     client,
		pullSecret: kube.NewImagePullApplicator(kube.NewSecretApplicator(client)),
	}
}

// NewManagerFromFlags returns a *Manager initialized from a
// registry.AuthorizedFlags.
func NewManagerFromFlags(client kubernetes.Interface, name, namespace string, flags registry.AuthorizedFlags) *Manager {
	return NewManager(client, name, namespace, flags.Username, flags.Password, flags.Endpoint.String())
}

// CreateOrUpdate creates the pull secret and its namespace if they don't
// exist, or updates the pull secret if it exists.
func (m *Manager) CreateOrUpdate(ctx context.Context) error {
	_, err := m.client.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: m.namespace,
		},
	}, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrap(err, fmt.Sprintf(errFmtCreateNamespace, m.namespace))
	}

	return m.pullSecret.Apply(
		ctx,
		m.name,
		m.namespace,
		m.username,
		m.password,
		m.endpoint,
	)
}
