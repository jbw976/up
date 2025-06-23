// Copyright 2025 Upbound Inc.
// All rights reserved

// Package spaces contains functions for ingress CA data handling
package spaces

import (
	"context"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up-sdk-go/apis/upbound/v1alpha1"
	"github.com/upbound/up/internal/version"
)

// ErrSpaceConnection is an error returned when the connection to the space,
// through the connect API, fails.
var ErrSpaceConnection = errors.New("failed to connect to space through the API client")

// SpaceIngress represents an ingress configuration for a space with host and CA data.
type SpaceIngress struct {
	Host   string
	CAData []byte
}

// IngressReader provides an interface for reading ingress configurations for spaces.
type IngressReader interface {
	Get(ctx context.Context, space v1alpha1.Space) (*SpaceIngress, error)
}

var (
	_ IngressReader = &configMapReader{}
	_ IngressReader = &mergingReader{}
	_ IngressReader = &IngressCache{}
)

// configMapReader reads ingress configuration from the space's ingress-public ConfigMap.
type configMapReader struct {
	bearer string
}

// NewConfigMapReader creates a new IngressReader that fetches ingress data
// from the space's ingress-public ConfigMap using the provided bearer token.
func NewConfigMapReader(bearer string) IngressReader {
	return &configMapReader{bearer: bearer}
}

func (c *configMapReader) Get(ctx context.Context, space v1alpha1.Space) (*SpaceIngress, error) {
	if space.Status.APIURL == "" {
		return nil, errors.New("API URL not defined on space")
	}

	cfg := &rest.Config{
		Host:        space.Status.APIURL,
		APIPath:     "/apis",
		UserAgent:   version.UserAgent(),
		BearerToken: c.bearer,
	}

	connectClient, err := client.New(cfg, client.Options{})
	if err != nil {
		return nil, ErrSpaceConnection
	}

	var ingressPublic corev1.ConfigMap
	if err := connectClient.Get(ctx, types.NamespacedName{Namespace: "upbound-system", Name: "ingress-public"}, &ingressPublic); err != nil {
		return nil, ErrSpaceConnection
	}

	host, ok := ingressPublic.Data["ingress-host"]
	if !ok {
		return nil, errors.New(`"ingress-host" not found in public ingress configmap`)
	}
	caString, ok := ingressPublic.Data["ingress-ca"]
	if !ok {
		return nil, errors.New(`"ingress-ca" not found in public ingress configmap`)
	}
	if err := ensureCertificateAuthorityData(caString); err != nil {
		return nil, err
	}

	return &SpaceIngress{
		Host:   host,
		CAData: []byte(caString),
	}, nil
}

// mergingReader wraps another IngressReader and merges a custom CA bundle.
type mergingReader struct {
	wrap           IngressReader
	customCABundle string
}

// NewMergingReader creates a new IngressReader that wraps another reader and
// merges a custom CA bundle with the space's CA data.
func NewMergingReader(wrap IngressReader, customCABundle string) IngressReader {
	return &mergingReader{wrap: wrap, customCABundle: customCABundle}
}

func (m *mergingReader) Get(ctx context.Context, space v1alpha1.Space) (*SpaceIngress, error) {
	base, err := m.wrap.Get(ctx, space)
	if err != nil {
		return nil, err
	}

	merged, err := mergeCACertificates(m.customCABundle, base.CAData)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to merge CA certificates for space %s", space.GetName())
	}

	return &SpaceIngress{
		Host:   base.Host,
		CAData: merged,
	}, nil
}

// IngressCache caches ingress results from a wrapped IngressReader.
type IngressCache struct {
	wrap      IngressReader
	mu        sync.RWMutex
	ingresses map[types.NamespacedName]SpaceIngress
}

// NewCachedReader creates a new cached IngressReader that wraps another reader.
// The returned reader caches ingress results to avoid repeated lookups.
func NewCachedReader(wrap IngressReader) *IngressCache {
	return &IngressCache{
		wrap:      wrap,
		ingresses: make(map[types.NamespacedName]SpaceIngress),
	}
}

// Get retrieves the ingress configuration for the given space, using the cache if available.
// If the ingress is not cached, it delegates to the wrapped IngressReader and stores the result.
func (c *IngressCache) Get(ctx context.Context, space v1alpha1.Space) (*SpaceIngress, error) {
	nsn := types.NamespacedName{Name: space.Name, Namespace: space.Namespace}

	c.mu.RLock()
	if ingress, ok := c.ingresses[nsn]; ok {
		c.mu.RUnlock()
		return &ingress, nil
	}
	c.mu.RUnlock()

	ingress, err := c.wrap.Get(ctx, space)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.ingresses[nsn] = *ingress
	c.mu.Unlock()

	return ingress, nil
}
