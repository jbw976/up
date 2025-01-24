// Copyright 2025 Upbound Inc.
// All rights reserved

// Package kube contains helpers for working with Kubernetes.
package kube

import (
	"context"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

// GetIngressHost returns the ingress host of the Spaces cfg points to. If the
// ingress is not configured, it returns an empty string.
func GetIngressHost(ctx context.Context, cl corev1client.ConfigMapsGetter) (host string, ca []byte, err error) {
	mxpConfig, err := cl.ConfigMaps("upbound-system").Get(ctx, "ingress-public", metav1.GetOptions{})
	if err != nil {
		return "", nil, err
	}

	host = mxpConfig.Data["ingress-host"]
	ca = []byte(mxpConfig.Data["ingress-ca"])
	return strings.TrimPrefix(host, "https://"), ca, nil
}
