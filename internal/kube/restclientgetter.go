// Copyright 2025 Upbound Inc.
// All rights reserved

package kube

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

type restClientGetter struct {
	namespace string
	config    *rest.Config
}

func NewRESTClientGetter(config *rest.Config, namespace string) *restClientGetter {
	return &restClientGetter{
		namespace: namespace,
		config:    config,
	}
}

// ToRESTConfig returns the underlying REST config.
func (c *restClientGetter) ToRESTConfig() (*rest.Config, error) {
	return c.config, nil
}

// ToDiscoveryClient builds a new discovery client.
func (c *restClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	config, err := c.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	// NOTE(hasheddan): these values match the burst and QPS values in kubectl.
	// xref: https://github.com/kubernetes/kubernetes/pull/105520
	config.Burst = 300
	config.QPS = 50
	discoveryClient, _ := discovery.NewDiscoveryClientForConfig(config)
	return memory.NewMemCacheClient(discoveryClient), nil
}

// ToRESTMapper builds a new REST mapper.
func (c *restClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	discoveryClient, err := c.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
	expander := restmapper.NewShortcutExpander(mapper, discoveryClient, nil)
	return expander, nil
}

// ToRawKubeConfigLoader loads a new raw kubeconfig.
func (c *restClientGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	overrides := &clientcmd.ConfigOverrides{ClusterDefaults: clientcmd.ClusterDefaults}
	overrides.Context.Namespace = c.namespace
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
}
