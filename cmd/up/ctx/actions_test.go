// Copyright 2025 Upbound Inc.
// All rights reserved

package ctx

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/runtime"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/upbound/up/internal/spaces"
	"github.com/upbound/up/internal/upbound"
)

func TestDisconnectedSpaceGetKubeconfig(t *testing.T) {
	t.Parallel()

	spaceKubeConfig := &clientcmdapi.Config{
		CurrentContext: "space",
		Contexts: map[string]*clientcmdapi.Context{
			"space": {Cluster: "upbound", AuthInfo: "upbound"},
		},
		Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "https://ingress", CertificateAuthorityData: []uint8{1, 2, 3}}},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token"}},
	}

	s := &DisconnectedSpace{
		BaseKubeconfig: spaceKubeConfig,
		Ingress: spaces.SpaceIngress{
			Host:   "ingress",
			CAData: []byte{1, 2, 3},
		},
	}

	spaceExtension := upbound.NewDisconnectedV1Alpha1SpaceExtension("space")
	extensionMap := map[string]runtime.Object{upbound.ContextExtensionKeySpace: spaceExtension}
	wantConf := &clientcmdapi.Config{
		APIVersion:     "v1",
		Kind:           "Config",
		CurrentContext: "upbound",
		Contexts: map[string]*clientcmdapi.Context{
			"upbound": {Cluster: "upbound", AuthInfo: "upbound", Extensions: extensionMap},
		},
		Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "https://ingress", CertificateAuthorityData: []uint8{1, 2, 3}}},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token"}},
	}

	got, err := s.GetKubeconfig()
	if diff := cmp.Diff(nil, err); diff != "" {
		t.Fatalf("GetKubeconfig(...): -want err, +got err:\n%s", diff)
	}

	raw, err := got.RawConfig()
	if diff := cmp.Diff(nil, err); diff != "" {
		t.Fatalf("RawConfig(...): -want err, +got err:\n%s", diff)
	}

	if diff := cmp.Diff(wantConf, &raw); diff != "" {
		t.Errorf("GetKubeconfig(...): -want conf, +got conf:\n%s", diff)
	}
}

func TestCloudSpaceGetKubeconfig(t *testing.T) {
	t.Parallel()

	s := &CloudSpace{
		name: "my-space",
		Org: Organization{
			Name: "my-org",
		},
		Ingress: spaces.SpaceIngress{
			Host:   "ingress",
			CAData: []byte{1, 2, 3},
		},
		AuthInfo: &clientcmdapi.AuthInfo{Token: "token"},
	}

	spaceExtension := upbound.NewCloudV1Alpha1SpaceExtension("my-org", "my-space")
	extensionMap := map[string]runtime.Object{upbound.ContextExtensionKeySpace: spaceExtension}
	wantConf := &clientcmdapi.Config{
		APIVersion:     "v1",
		Kind:           "Config",
		CurrentContext: "upbound",
		Contexts: map[string]*clientcmdapi.Context{
			"upbound": {Cluster: "upbound", AuthInfo: "upbound", Extensions: extensionMap},
		},
		Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "https://ingress", CertificateAuthorityData: []uint8{1, 2, 3}}},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token"}},
	}

	got, err := s.GetKubeconfig()
	if diff := cmp.Diff(nil, err); diff != "" {
		t.Fatalf("GetKubeconfig(...): -want err, +got err:\n%s", diff)
	}

	raw, err := got.RawConfig()
	if diff := cmp.Diff(nil, err); diff != "" {
		t.Fatalf("RawConfig(...): -want err, +got err:\n%s", diff)
	}

	if diff := cmp.Diff(wantConf, &raw); diff != "" {
		t.Errorf("GetKubeconfig(...): -want conf, +got conf:\n%s", diff)
	}
}

func TestGroupGetKubeconfig(t *testing.T) {
	t.Parallel()

	g := &Group{
		Name: "my-group",
		Space: &CloudSpace{
			name: "my-space",
			Org: Organization{
				Name: "my-org",
			},
			Ingress: spaces.SpaceIngress{
				Host:   "ingress",
				CAData: []byte{1, 2, 3},
			},
			AuthInfo: &clientcmdapi.AuthInfo{Token: "token"},
		},
	}

	spaceExtension := upbound.NewCloudV1Alpha1SpaceExtension("my-org", "my-space")
	extensionMap := map[string]runtime.Object{upbound.ContextExtensionKeySpace: spaceExtension}
	wantConf := &clientcmdapi.Config{
		APIVersion:     "v1",
		Kind:           "Config",
		CurrentContext: "upbound",
		Contexts: map[string]*clientcmdapi.Context{
			"upbound": {Namespace: "my-group", Cluster: "upbound", AuthInfo: "upbound", Extensions: extensionMap},
		},
		Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "https://ingress", CertificateAuthorityData: []uint8{1, 2, 3}}},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token"}},
	}

	got, err := g.GetKubeconfig()
	if diff := cmp.Diff(nil, err); diff != "" {
		t.Fatalf("GetKubeconfig(...): -want err, +got err:\n%s", diff)
	}

	raw, err := got.RawConfig()
	if diff := cmp.Diff(nil, err); diff != "" {
		t.Fatalf("RawConfig(...): -want err, +got err:\n%s", diff)
	}

	if diff := cmp.Diff(wantConf, &raw); diff != "" {
		t.Errorf("GetKubeconfig(...): -want conf, +got conf:\n%s", diff)
	}
}

func TestControlPlaneGetKubeconfig(t *testing.T) {
	t.Parallel()

	ctp := &ControlPlane{
		Name: "my-ctp",
		Group: Group{
			Name: "my-group",
			Space: &CloudSpace{
				name: "my-space",
				Org: Organization{
					Name: "my-org",
				},
				Ingress: spaces.SpaceIngress{
					Host:   "ingress",
					CAData: []byte{1, 2, 3},
				},
				AuthInfo: &clientcmdapi.AuthInfo{Token: "token"},
			},
		},
	}

	spaceExtension := upbound.NewCloudV1Alpha1SpaceExtension("my-org", "my-space")
	extensionMap := map[string]runtime.Object{upbound.ContextExtensionKeySpace: spaceExtension}
	wantConf := &clientcmdapi.Config{
		APIVersion:     "v1",
		Kind:           "Config",
		CurrentContext: "upbound",
		Contexts: map[string]*clientcmdapi.Context{
			"upbound": {Namespace: "default", Cluster: "upbound", AuthInfo: "upbound", Extensions: extensionMap},
		},
		Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "https://ingress/apis/spaces.upbound.io/v1beta1/namespaces/my-group/controlplanes/my-ctp/k8s", CertificateAuthorityData: []uint8{1, 2, 3}}},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token"}},
	}

	got, err := ctp.GetKubeconfig()
	if diff := cmp.Diff(nil, err); diff != "" {
		t.Fatalf("GetKubeconfig(...): -want err, +got err:\n%s", diff)
	}

	raw, err := got.RawConfig()
	if diff := cmp.Diff(nil, err); diff != "" {
		t.Fatalf("RawConfig(...): -want err, +got err:\n%s", diff)
	}

	if diff := cmp.Diff(wantConf, &raw); diff != "" {
		t.Errorf("GetKubeconfig(...): -want conf, +got conf:\n%s", diff)
	}
}
