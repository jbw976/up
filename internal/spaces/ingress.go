// Copyright 2024 Upbound Inc
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

package spaces

import (
	"context"
	"crypto/x509"
	"encoding/pem"
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

type SpaceIngress struct {
	Host   string
	CAData []byte
}

type IngressReader interface {
	Get(ctx context.Context, space v1alpha1.Space) (ingress *SpaceIngress, err error)
}

var _ IngressReader = &ingressCache{}

type ingressCache struct {
	mu sync.RWMutex

	// ingresses contains a map of a space's namespaced name to its
	// corresponding ingress configuration
	ingresses map[types.NamespacedName]SpaceIngress

	bearer string
}

func NewCachedReader(bearer string) *ingressCache {
	return &ingressCache{
		ingresses: make(map[types.NamespacedName]SpaceIngress),
		bearer:    bearer,
	}
}

func (c *ingressCache) cacheGet(space v1alpha1.Space) (SpaceIngress, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	nsn := types.NamespacedName{Name: space.Name, Namespace: space.Namespace}
	ingress, found := c.ingresses[nsn]
	return ingress, found
}

func (c *ingressCache) cachePut(space v1alpha1.Space, ingress SpaceIngress) {
	c.mu.Lock()
	defer c.mu.Unlock()

	nsn := types.NamespacedName{Name: space.Name, Namespace: space.Namespace}
	c.ingresses[nsn] = ingress
}

func (c *ingressCache) Get(ctx context.Context, space v1alpha1.Space) (ingress *SpaceIngress, err error) {
	// cache hit
	if i, ok := c.cacheGet(space); ok {
		return &i, nil
	}

	ingress = &SpaceIngress{}
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

	var ok bool
	if ingress.Host, ok = ingressPublic.Data["ingress-host"]; !ok {
		return nil, errors.Wrap(err, `"ingress-host" not found in public ingress configmap`)
	}
	if caString, ok := ingressPublic.Data["ingress-ca"]; !ok {
		return nil, errors.Wrap(err, `"ingress-ca" not found in public ingress configmap`)
	} else if err = ensureCertificateAuthorityData(caString); err != nil {
		return nil, err
	} else {
		ingress.CAData = []byte(caString)
	}

	c.cachePut(space, *ingress)

	return ingress, err
}

func ensureCertificateAuthorityData(tlsCert string) error {
	block, _ := pem.Decode([]byte(tlsCert))
	if block == nil || block.Type != "CERTIFICATE" {
		return errors.New("CA string does not contain PEM certificate data")
	}

	_, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return errors.Wrap(err, "CA cannot be parsed to x509 certificate")
	}
	return nil
}
