// Copyright 2025 Upbound Inc.
// All rights reserved

package ctx

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// GetKubeconfig upserts the "upbound" kubeconfig context and cluster to the chosen
// kubeconfig, pointing to the space.
func (s *CloudSpace) GetKubeconfig() (*clientcmdapi.Config, error) {
	return getSpaceKubeconfig(s)
}

// GetKubeconfig upserts the "upbound" kubeconfig context and cluster to the chosen
// kubeconfig, pointing to the space.
func (s *DisconnectedSpace) GetKubeconfig() (*clientcmdapi.Config, error) {
	return getSpaceKubeconfig(s)
}

func getSpaceKubeconfig(s Space) (*clientcmdapi.Config, error) {
	config, err := s.BuildKubeconfig(types.NamespacedName{})
	if err != nil {
		return nil, err
	}
	raw, err := config.RawConfig()
	if err != nil {
		return nil, err
	}

	return &raw, nil
}

// GetKubeconfig upserts the "upbound" kubeconfig context and cluster to the chosen
// kubeconfig, pointing to the group.
func (g *Group) GetKubeconfig() (*clientcmdapi.Config, error) {
	config, err := g.Space.BuildKubeconfig(types.NamespacedName{Namespace: g.Name})
	if err != nil {
		return nil, err
	}
	raw, err := config.RawConfig()
	if err != nil {
		return nil, err
	}

	return &raw, nil
}

// GetKubeconfig upserts a controlplane context and cluster to the chosen kubeconfig.
func (ctp *ControlPlane) GetKubeconfig() (*clientcmdapi.Config, error) {
	config, err := ctp.Group.Space.BuildKubeconfig(ctp.NamespacedName())
	if err != nil {
		return nil, err
	}
	raw, err := config.RawConfig()
	if err != nil {
		return nil, err
	}

	return &raw, nil
}

func acceptState(s Accepting, navCtx *navContext) (msg string, err error) {
	config, err := s.GetKubeconfig()
	if err != nil {
		return "", err
	}

	if err := navCtx.contextWriter.Write(config); err != nil {
		return "", err
	}

	return fmt.Sprintf(contextSwitchedFmt, withUpboundPrefix(s.Breadcrumbs().styledString())), nil
}
