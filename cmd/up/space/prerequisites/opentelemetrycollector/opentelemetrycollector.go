// Copyright 2025 Upbound Inc.
// All rights reserved

// Package opentelemetrycollector provides a Helm manager for the opentelemetry-operator chart.
package opentelemetrycollector

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/pterm/pterm"
	corev1 "k8s.io/api/core/v1"
	apixv1client "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/util/podutils"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/install/helm"
)

//nolint:gochecknoglobals // global variables for otel initialization.
var (
	// Set image used to contrib to cover more exporters.
	values = map[string]any{
		"manager": map[string]any{
			"collectorImage": map[string]any{
				"repository": "otel/opentelemetry-collector-contrib",
			},
		},
	}
	otelMgrURL, _ = url.Parse("https://open-telemetry.github.io/opentelemetry-helm-charts")
)

const (
	chartName      = "opentelemetry-operator"
	chartNamespace = chartName

	// Chart version to be installed.
	version = "0.78.2"

	otelCollectorCRD = "opentelemetrycollectors.opentelemetry.io"

	errFmtCreateHelmManager = "failed to create helm manager for %s"
	errFmtCreateK8sClient   = "failed to create kubernetes client for helm chart %s"
	errFmtCreateNamespace   = "failed to create namespace %s"
)

// Operator represents a Helm manager.
type Operator struct {
	mgr       install.Manager
	crdclient *apixv1client.ApiextensionsV1Client
	kclient   kubernetes.Interface
}

// New constructs a new OpenTelemetryCollectorMgr instance that can used to install the
// opentelemetry-operator chart.
func New(config *rest.Config) (*Operator, error) {
	mgr, err := helm.NewManager(config,
		chartName,
		otelMgrURL,
		helm.WithNamespace(chartNamespace),
	)
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

	return &Operator{
		mgr:       mgr,
		crdclient: crdclient,
		kclient:   kclient,
	}, nil
}

// GetName returns the name of the opentelemetry-operator chart.
func (o *Operator) GetName() string {
	return chartName
}

// Install performs a Helm install of the chart.
func (o *Operator) Install() error {
	installed, err := o.IsInstalled()
	if err != nil {
		return err
	}
	if installed {
		// nothing to do
		return nil
	}

	// create namespace before creating chart.
	_, err = o.kclient.CoreV1().
		Namespaces().
		Create(context.Background(),
			&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: chartNamespace,
				},
			}, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrap(err, fmt.Sprintf(errFmtCreateNamespace, chartNamespace))
	}

	if err = o.mgr.Install(version, values); err != nil {
		return err
	}

	// wait until the operator pod is ready because Spaces
	// OpenTelemetryCollector needs the mutating webhook to be ready
	// to not fail the installation.
	return o.waitUntilReady()
}

// waitUntilReady waits until the opentelemetry-operator pod is ready, or
// until the timeout.
func (o *Operator) waitUntilReady() error {
	return errors.Wrap(wait.PollUntilContextTimeout(context.Background(), 2*time.Second, 10*time.Minute, true, func(ctx context.Context) (bool, error) {
		pods, err := o.kclient.CoreV1().Pods(chartNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: "app.kubernetes.io/name=opentelemetry-operator",
		})
		if err != nil {
			pterm.Info.Printf("Cannot list pods in namespace %q: %v \n", chartNamespace, err)
			return false, err
		}
		if pods == nil || len(pods.Items) != 1 {
			pterm.Info.Println("Cannot find the opentelemetry-operator pod...")
			return false, err
		}
		if podutils.IsPodReady(&pods.Items[0]) {
			return true, nil
		}
		return false, nil
	}), "failed to wait for opentelemetry-operator pod to be ready")
}

// IsInstalled checks if opentelemetry operator has been installed in the target cluster.
func (o *Operator) IsInstalled() (bool, error) {
	_, err := o.crdclient.
		CustomResourceDefinitions().
		Get(
			context.Background(),
			otelCollectorCRD,
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
