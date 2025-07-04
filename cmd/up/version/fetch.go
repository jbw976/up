// Copyright 2025 Upbound Inc.
// All rights reserved

// Package version contains common functions to get versions
package version

import (
	"context"
	"strings"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/upbound"
)

const (
	errFetchDeployment = "could not fetch deployments"
)

// FetchCrossplaneVersion initializes a Kubernetes client and fetches
// and returns the version of the Crossplane deployment. If the version
// does not have a leading 'v', it prepends it.
func FetchCrossplaneVersion(ctx context.Context, clientset kubernetes.Clientset) (string, error) {
	var version string

	deployments, err := clientset.AppsV1().Deployments("").List(ctx, v1.ListOptions{
		LabelSelector: "app=crossplane",
	})
	if err != nil {
		return "", errors.Wrap(err, errFetchDeployment)
	}

	for _, deployment := range deployments.Items {
		v, ok := deployment.Labels["app.kubernetes.io/version"]
		if ok {
			if !strings.HasPrefix(v, "v") {
				version = "v" + v
			}
			return version, nil
		}

		if len(deployment.Spec.Template.Spec.Containers) > 0 {
			image := deployment.Spec.Template.Spec.Containers[0].Image
			parts := strings.Split(image, ":")
			if len(parts) > 1 {
				imageTag := parts[1]
				if !strings.HasPrefix(imageTag, "v") {
					imageTag = "v" + imageTag
				}
				return imageTag, nil
			}
		}
	}

	return "", errors.New("Crossplane version or image tag not found")
}

// FetchSpacesVersion initializes a Kubernetes client and fetches
// and returns the version of the spaces-controller deployment.
func FetchSpacesVersion(ctx context.Context, context *clientcmdapi.Context, clientset kubernetes.Clientset) (string, error) {
	deployments, err := clientset.AppsV1().Deployments("").List(ctx, v1.ListOptions{
		LabelSelector: "app=spaces-controller",
	})
	if err != nil {
		return "", errors.Wrap(err, errFetchDeployment)
	}

	for _, deployment := range deployments.Items {
		v, ok := deployment.Labels["app.kubernetes.io/version"]
		if ok {
			return v, nil
		}
	}

	ext, err := upbound.GetSpaceExtension(context)
	if err == nil && ext != nil && ext.Spec.Cloud != nil {
		return "Upbound Cloud Managed", nil
	}

	return "", errors.New("spaces-controller version not found")
}
