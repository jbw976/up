// Copyright 2025 Upbound Inc.
// All rights reserved

package webui

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/pkg/browser"
	"github.com/pterm/pterm"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/upbound/up/internal/ctp"
	"github.com/upbound/up/internal/license"
	"github.com/upbound/up/internal/upbound"
)

const (
	namespace   = "crossplane-system"
	svcName     = "webui"
	svcPortName = "http"

	errCreateClient        = "failed to create client"
	errCreatePortForward   = "failed to create port-forward"
	errFmtGetContainerPort = "failed to get container port for pod %q"
	errFmtGetService       = "failed to get service %q"
	errFmtGetServicePod    = "failed to get pod for service %q"
	errFmtServicePods      = "no pods match selector for service: %s"
	errFmtServiceSelector  = "service %q has no selector"
	errGetControllerClient = "failed to get controller-runtime client"
	errGetKubeClient       = "failed to get kube client"
	errGetListenerAddr     = "failed to get listener addr"
	errGetRoundTripper     = "failed to get roundtripper"
	errGetServicePort      = "failed to get service port"
	errLicenseNotFound     = "License not found. Verify your kubeconfig is pointing at a UXP control plane."
	errListen              = "failed to listen on an available port"
	errListPods            = "failed to list pods"
	errMakePortForwarder   = "failed to make port-forwarder"
	errParseLabelSelector  = "failed to parse label selector"
	errStartPortForward    = "failed to start port-forward"
	errFmtServiceNotFound  = "Service %q not found. Verify your kubeconfig is pointing at a UXP control plane and the web UI is enabled"
)

// openCmd opens the UXP web UI in a browser.
type openCmd struct {
	Host    string `default:"localhost"                                                    help:"Host to listen on for port-forward."`
	Port    int    `help:"Port to listen on for port-forward (0 for automatic selection)."`
	Browser bool   `default:"true"                                                         help:"Open the web UI in a browser window." negatable:""`
}

func (c *openCmd) Run(ctx context.Context, upCtx *upbound.Context, cfg *rest.Config) error {
	cl, err := client.New(cfg, client.Options{})
	if err != nil {
		return errors.Wrap(err, errCreateClient)
	}

	err = license.CheckUXPv2(ctx, cl, true)
	if errors.Is(err, license.ErrNotFound) {
		return errors.New(errLicenseNotFound)
	}
	if err != nil {
		return err
	}

	// Try to determine if the kubeconfig context is a local dev cluster with an
	// ingress. If it is then we don't need to port forward.
	if url, ok := c.localClusterIngressURL(ctx, upCtx); ok {
		return c.open(url)
	}

	pf, url, ready, err := c.portForwarder(ctx, cfg, cl)
	if err != nil {
		return errors.Wrap(err, errCreatePortForward)
	}

	pfErr := make(chan error)
	go func() {
		pfErr <- pf.ForwardPorts()
	}()

	select {
	case err := <-pfErr:
		return errors.Wrap(err, errStartPortForward)
	case <-ready:
		pterm.Println("Port forwarding started; interrupt to stop")
	}

	if err := c.open(url); err != nil {
		// Add a blank line to distinguish the error message from regular
		// output.
		pterm.Println()
		pterm.Println(err)
		// Continue executing. The port-forward is reachable from the URL
		// printed earlier.
	}

	<-ctx.Done()
	return nil
}

// localClusterIngressURL returns the URL to the Web UI ingress if the user's
// kubeconfig context is a local dev cluster with ingress enabled.
func (c *openCmd) localClusterIngressURL(ctx context.Context, upCtx *upbound.Context) (string, bool) {
	cfg, err := upCtx.GetRawKubeconfig()
	if err != nil {
		return "", false
	}

	// TODO(adamwg): This is a bad heuristic for whether the context is a local
	// dev cluster, but it works well enough for our purposes.
	if !strings.HasPrefix(cfg.CurrentContext, "kind-up-") {
		return "", false
	}
	clusterName := strings.TrimPrefix(cfg.CurrentContext, "kind-")
	cluster, found, err := ctp.FindDevControlPlane(ctx, upCtx,
		ctp.FindWithControlPlaneName(clusterName),
		ctp.FindForceLocal(true),
	)
	if err != nil || !found {
		return "", false
	}

	icluster, ok := cluster.(ctp.IngressControlPlane)
	if !ok {
		return "", false
	}

	return icluster.WebUIAddress(), true
}

func (c *openCmd) open(url string) error {
	pterm.Printfln("The web UI is available at: %s", url)
	if c.Browser {
		if err := browser.OpenURL(url); err != nil {
			return errors.Wrap(err, "failed to open web UI in browser")
		}
	}
	return nil
}

// portForwarder returns a *portforward.PortForwarder to a web-ui pod, the
// URL it listens on, and a channel that is closed when the PortForwarder is
// ready.
func (c *openCmd) portForwarder(ctx context.Context, cfg *rest.Config, cl client.Client) (*portforward.PortForwarder, string, chan struct{}, error) {
	localPort, err := c.localPort(ctx)
	if err != nil {
		return nil, "", nil, err
	}

	svc := &corev1.Service{}
	if err := cl.Get(ctx, types.NamespacedName{Namespace: namespace, Name: svcName}, svc); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, "", nil, fmt.Errorf(errFmtServiceNotFound, svcName) //nolint:staticcheck // Error message intentionally has punctuation and is capitalized
		}
		return nil, "", nil, errors.Wrap(err, fmt.Sprintf(errFmtGetService, svcName))
	}

	svcPort, err := c.servicePort(svc, svcPortName)
	if err != nil {
		return nil, "", nil, errors.Wrap(err, errGetServicePort)
	}

	pod, err := c.servicePod(ctx, cl, svc)
	if err != nil {
		return nil, "", nil, errors.Wrap(err, fmt.Sprintf(errFmtGetServicePod, svcName))
	}

	cPort, err := c.containerPort(pod, svcPort.TargetPort)
	if err != nil {
		return nil, "", nil, errors.Wrap(err, fmt.Sprintf(errFmtGetContainerPort, pod.Name))
	}

	kube, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, "", nil, errors.Wrap(err, errGetKubeClient)
	}

	req := kube.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(namespace).
		Name(pod.Name).
		SubResource("portforward")

	transport, upgrader, err := spdy.RoundTripperFor(cfg)
	if err != nil {
		return nil, "", nil, errors.Wrap(err, errGetRoundTripper)
	}

	readyCh := make(chan struct{})
	pf, err := portforward.NewOnAddresses(
		spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, req.URL()),
		[]string{c.Host},
		[]string{fmt.Sprintf("%d:%d", localPort, cPort)},
		ctx.Done(),
		readyCh,
		io.Discard,
		os.Stderr,
	)
	if err != nil {
		return nil, "", nil, errors.Wrap(err, errMakePortForwarder)
	}

	url := fmt.Sprintf("http://%s", net.JoinHostPort(c.Host, strconv.Itoa(localPort)))

	return pf, url, readyCh, nil
}

// localPort returns an unused local port.
func (c *openCmd) localPort(ctx context.Context) (int, error) {
	if c.Port != 0 {
		return c.Port, nil
	}

	lc := net.ListenConfig{}
	listener, err := lc.Listen(ctx, "tcp", net.JoinHostPort(c.Host, "0"))
	if err != nil {
		return 0, errors.Wrap(err, errListen)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			pterm.Println(fmt.Sprintf("Error closing listener: %s", err))
		}
	}()

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, errors.New(errGetListenerAddr)
	}
	return addr.Port, nil
}

// servicePort returns a *corev1.ServicePort on svc that matches the provided
// name.
func (c *openCmd) servicePort(svc *corev1.Service, name string) (*corev1.ServicePort, error) {
	for _, p := range svc.Spec.Ports {
		if p.Name == name {
			return &p, nil
		}
	}
	return nil, errors.New("not found")
}

// servicePod returns the first pod that matches the selector on svc.
func (c *openCmd) servicePod(ctx context.Context, cl client.Client, svc *corev1.Service) (*corev1.Pod, error) {
	selector := svc.Spec.Selector
	if len(selector) == 0 {
		return nil, fmt.Errorf(errFmtServiceSelector, svc.Name)
	}

	labelSelector, err := labels.Parse(metav1.FormatLabelSelector(&metav1.LabelSelector{MatchLabels: selector}))
	if err != nil {
		return nil, errors.Wrap(err, errParseLabelSelector)
	}

	pods := &corev1.PodList{}
	if err := cl.List(ctx, pods, client.MatchingLabelsSelector{Selector: labelSelector}); err != nil {
		return nil, errors.Wrap(err, errListPods)
	}
	if len(pods.Items) == 0 {
		return nil, fmt.Errorf(errFmtServicePods, svc.Name)
	}

	return &pods.Items[0], nil
}

// containerPort returns the container port on pod whose name or containerPort
// matches targetPort.
func (c *openCmd) containerPort(pod *corev1.Pod, targetPort intstr.IntOrString) (int, error) {
	for _, container := range pod.Spec.Containers {
		for _, p := range container.Ports {
			if (targetPort.Type == intstr.String && p.Name == targetPort.StrVal) ||
				(targetPort.Type == intstr.Int && p.ContainerPort == targetPort.IntVal) {
				return int(p.ContainerPort), nil
			}
		}
	}
	return 0, errors.New("not found")
}
