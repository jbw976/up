// Copyright 2025 Upbound Inc.
// All rights reserved

// Package defaults contains defaults for Spaces.
package defaults

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/upbound/up/internal/upterm"
)

// CloudType is a type of (usually cloud-hosted) Kubernetes cluster.
type CloudType string

// CloudConfig contains cloud-specific configuration settings for Spaces.
type CloudConfig struct {
	PublicIngress bool
}

const (
	// AmazonEKS is the EKS type of cluster.
	AmazonEKS CloudType = "eks"
	// AzureAKS is the AKS type of cluster.
	AzureAKS CloudType = "aks"
	// Generic is a generic cluster.
	Generic CloudType = "generic"
	// GoogleGKE is the GKE type of cluster.
	GoogleGKE CloudType = "gke"
	// Kind is the kind type of cluster.
	Kind CloudType = "kind"

	// ClusterTypeStr is the configuration key for the cloud type.
	ClusterTypeStr = "clusterType"
)

// Defaults returns the defaults for a given type of cluster.
func (ct *CloudType) Defaults() CloudConfig {
	return CloudConfig{
		PublicIngress: *ct != Generic && *ct != Kind,
	}
}

// GetConfig returns the Spaces configuration to use for a cluster based on its
// type, inferred from its Kubernetes client.
func GetConfig(kClient kubernetes.Interface, override string, p upterm.Printer) (*CloudConfig, error) {
	if kClient == nil {
		return nil, errors.New("no kubernetes client")
	}
	var cloud CloudType
	if override != "" {
		cloud = CloudType(strings.ToLower(override))
	} else {
		cloud = detectKubernetes(kClient)
	}
	if cloud == Generic || cloud == Kind {
		p.PrintInfo(fmt.Sprintf("Setting defaults for vanilla Kubernetes (type %s)", cloud))
	} else {
		p.PrintInfo(fmt.Sprintf("Applying settings for Managed Kubernetes on %s", strings.ToUpper(string(cloud))))
	}
	return &CloudConfig{
		PublicIngress: cloud.Defaults().PublicIngress,
	}, nil
}

// detectKubernetes looks at a nodes provider to determine what type of cluster
// is running. Since Spaces doesn't directly use Node objects, requiring Nodes
// to use the installer would be incorrect. This is a "best effort" attempt to
// add some CLI sugar, so reacting to an error seems suboptimal, especially if the
// installer doesn't have RBAC permissions to list nodes.
func detectKubernetes(kClient kubernetes.Interface) CloudType {
	// EKS and Kind are _harder_ to detect based on version, so look at node labels.
	ctx := context.Background()
	if nodes, err := kClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{}); err == nil {
		for _, n := range nodes.Items {
			providerPrefix := strings.Split(n.Spec.ProviderID, "://")[0]
			switch providerPrefix {
			case "azure":
				return AzureAKS
			case "aws":
				return AmazonEKS
			case "gce":
				return GoogleGKE
			case "kind":
				return Kind
			}
		}
	}

	return Generic
}
