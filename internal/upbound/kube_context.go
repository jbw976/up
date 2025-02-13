// Copyright 2025 Upbound Inc.
// All rights reserved

package upbound

import (
	"encoding/json"
	"strings"

	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/profile"
	"github.com/upbound/up/internal/version"
)

const (
	// ContextExtensionKeySpace is the key used in a context extension for a
	// space extension.
	ContextExtensionKeySpace = "spaces.upbound.io/space"
)

// HasValidContext returns true if the kube configuration attached to the
// context is valid and usable.
func (c *Context) HasValidContext() bool {
	// todo(redbackthomson): Add support for overriding current context as part
	// of CLI args
	config, err := c.Kubecfg.RawConfig()
	if err != nil {
		return false
	}

	return clientcmd.ConfirmUsable(config, "") == nil
}

// GetKubeconfig returns a Kubernetes rest config for the current context.
func (c *Context) GetKubeconfig() (*rest.Config, error) {
	r, err := c.Kubecfg.ClientConfig()
	if err != nil {
		return nil, err
	}
	r.UserAgent = version.UserAgent()

	return r, nil
}

// GetRawKubeconfig returns the raw kubeconfig for the current context.
func (c *Context) GetRawKubeconfig() (clientcmdapi.Config, error) {
	return c.Kubecfg.RawConfig()
}

// BuildCurrentContextClient creates a K8s client using the current Kubeconfig
// defaulting to the current Kubecontext.
func (c *Context) BuildCurrentContextClient() (client.Client, error) {
	rest, err := c.GetKubeconfig()
	if err != nil {
		return nil, errors.Wrap(err, "unable to get kube config")
	}

	// todo(redbackthomson): Delete once spaces-api is able to accept protobuf
	// requests
	rest.ContentConfig.ContentType = "application/json"

	sc, err := client.New(rest, client.Options{})
	if err != nil {
		return nil, errors.Wrap(err, "error creating kube client")
	}
	return sc, nil
}

// GetCurrentContextName returns the name of the current kubeconfig context.
func (c *Context) GetCurrentContextName() (string, error) {
	config, err := c.Kubecfg.RawConfig()
	if err != nil {
		return "", err
	}

	return config.CurrentContext, nil
}

// GetCurrentContext returns the current kubeconfig context along with the
// cluster and auth to which it refers.
func (c *Context) GetCurrentContext() (context *clientcmdapi.Context, cluster *clientcmdapi.Cluster, auth *clientcmdapi.AuthInfo, exists bool) {
	config, err := c.Kubecfg.RawConfig()
	if err != nil {
		return nil, nil, nil, false
	}

	current := config.CurrentContext
	if current == "" {
		return nil, nil, nil, false
	}

	context, exists = config.Contexts[current]
	if !exists {
		return nil, nil, nil, false
	}

	cluster, exists = config.Clusters[context.Cluster]

	if context.AuthInfo == "" {
		return context, cluster, nil, exists
	}

	auth, exists = config.AuthInfos[context.AuthInfo]
	return context, cluster, auth, exists
}

// GetCurrentContextNamespace returns the default namespace in the current
// kubeconfig context.
func (c *Context) GetCurrentContextNamespace() (string, error) {
	ns, _, err := c.Kubecfg.Namespace()
	return ns, err
}

// GetCurrentSpaceContextScope checks whether the current kubeconfig context
// refers to a Space or a Control Plane within a Space. It returns the ingress
// host for the Space. If the context refers to a Group, its Namespace is also
// returned. If the context refers to a Control Plane, a reference to the
// ControlPlane resource is also returned.
func (c *Context) GetCurrentSpaceContextScope() (ingressHost string, ctp types.NamespacedName, inSpace bool) {
	context, cluster, _, exists := c.GetCurrentContext()
	if !exists {
		return "", types.NamespacedName{}, false
	}

	if cluster == nil || cluster.Server == "" {
		return "", types.NamespacedName{}, false
	}

	base, nsn, inSpace := profile.ParseSpacesK8sURL(strings.TrimSuffix(cluster.Server, "/"))
	// we are inside a ctp scope
	if inSpace {
		return strings.TrimPrefix(base, "https://"), nsn, inSpace
	}

	ingressHost = strings.TrimPrefix(cluster.Server, "https://")

	// we aren't inside a group scope
	if context.Namespace == "" {
		return ingressHost, types.NamespacedName{}, true
	}

	return ingressHost, types.NamespacedName{Namespace: context.Namespace}, true
}

// GetSpaceExtension attempts to get the context space extension for the
// provided context, if it exists.
func GetSpaceExtension(context *clientcmdapi.Context) (extension *SpaceExtension, err error) {
	if context == nil {
		return nil, nil
	} else if ext, ok := context.Extensions[ContextExtensionKeySpace].(*runtime.Unknown); !ok {
		return nil, nil
	} else if err := json.Unmarshal(ext.Raw, &extension); err != nil {
		return nil, errors.New("unable to parse space extension to go struct")
	}
	return extension, nil
}
