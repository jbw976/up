// Copyright 2025 Upbound Inc.
// All rights reserved

package uxp

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apixv1client "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/cmd/up/uxp"
	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/install/helm"
)

var (
	chartName = "universal-crossplane"
	ns        = "upbound-system"
	// Chart version to be installed. universal-crossplane does not include a
	// v prefix.
	version = "1.15.2-up.1"

	xrdCRD = "compositeresourcedefinitions.apiextensions.crossplane.io"

	errFmtCreateNamespace   = "failed to create namespace %s"
	errFmtCreateHelmManager = "failed to create helm manager for %s"
	errFmtCreateK8sClient   = "failed to create kubernetes client for helm chart %s"
)

// UXP represents a Helm manager that enables installing the
// universal-crossplane helm chart.
type UXP struct {
	mgr       install.Manager
	crdclient *apixv1client.ApiextensionsV1Client
	kclient   kubernetes.Interface
}

// New constructs a new UXP instance that can used to install the
// universal-crossplane chart.
func New(config *rest.Config) (*UXP, error) {
	mgr, err := helm.NewManager(config,
		chartName,
		uxp.RepoURL,
		// The default namespace is upbound-system, but we set it in order to
		// be explicit.
		helm.WithNamespace(ns),
		helm.Wait())
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf(errFmtCreateHelmManager, chartName))
	}
	crdclient, err := apixv1client.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf(errFmtCreateK8sClient, chartName))
	}
	kclient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf(errFmtCreateK8sClient, chartName))
	}

	return &UXP{
		mgr:       mgr,
		crdclient: crdclient,
		kclient:   kclient,
	}, nil
}

// GetName returns the name of the universal-crossplane chart.
func (u *UXP) GetName() string {
	return chartName
}

// Install performs a Helm install of the chart.
func (u *UXP) Install() error {
	installed, err := u.IsInstalled()
	if err != nil {
		return err
	}
	if installed {
		// nothing to do
		return nil
	}
	// create namespace before creating chart.
	_, err = u.kclient.CoreV1().
		Namespaces().
		Create(context.Background(),
			&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: ns,
				},
			}, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrap(err, fmt.Sprintf(errFmtCreateNamespace, ns))
	}
	return u.mgr.Install(version, map[string]any{
		"args": []string{
			"--enable-usages",
			"--max-reconcile-rate=1000",
		},
		"resourcesCrossplane": map[string]any{
			"requests": map[string]any{
				"cpu":    "500m",
				"memory": "1Gi",
			},
			"limits": map[string]any{
				"cpu":    "1000m",
				"memory": "2Gi",
			},
		},
	})
}

// IsInstalled checks if UXP has been installed in the target cluster.
func (u *UXP) IsInstalled() (bool, error) {
	_, err := u.crdclient.
		CustomResourceDefinitions().
		Get(
			context.Background(),
			xrdCRD,
			metav1.GetOptions{},
		)
	if err == nil {
		return true, nil
	}
	if kerrors.IsNotFound(err) {
		return false, nil
	}
	return false, err
}
