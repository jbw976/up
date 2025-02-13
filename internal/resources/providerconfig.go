// Copyright 2025 Upbound Inc.
// All rights reserved

package resources

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
)

var (
	kind = "ProviderConfig"
	// ProviderConfigHelmGVK is the GroupVersionKind used for
	// provider-helm ProviderConfig.
	ProviderConfigHelmGVK = schema.GroupVersionKind{
		Group:   "helm.crossplane.io",
		Version: "v1beta1",
		Kind:    kind,
	}
	// ProviderConfigKubernetesGVK is the GroupVersionKind used for
	// provider-kubernetes ProviderConfig.
	ProviderConfigKubernetesGVK = schema.GroupVersionKind{
		Group:   "kubernetes.crossplane.io",
		Version: "v1alpha1",
		Kind:    kind,
	}
)

// ProviderConfig represents a Crossplane ProviderConfig.
type ProviderConfig struct {
	unstructured.Unstructured
}

// GetUnstructured returns the unstructured representation of the package.
func (p *ProviderConfig) GetUnstructured() *unstructured.Unstructured {
	return &p.Unstructured
}

// SetCredentialsSource for the Provider.
func (p *ProviderConfig) SetCredentialsSource(src xpv1.CredentialsSource) {
	_ = fieldpath.Pave(p.Object).SetValue("spec.credentials.source", src)
}
