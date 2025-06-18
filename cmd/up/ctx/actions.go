// Copyright 2025 Upbound Inc.
// All rights reserved

package ctx

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
)

// GetKubeconfig upserts the "upbound" kubeconfig context and cluster to the chosen
// kubeconfig, pointing to the space.
func (s *CloudSpace) GetKubeconfig() (clientcmd.ClientConfig, error) {
	return s.BuildKubeconfig(types.NamespacedName{})
}

// GetKubeconfig upserts the "upbound" kubeconfig context and cluster to the chosen
// kubeconfig, pointing to the space.
func (s *DisconnectedSpace) GetKubeconfig() (clientcmd.ClientConfig, error) {
	return s.BuildKubeconfig(types.NamespacedName{})
}

// GetKubeconfig upserts the "upbound" kubeconfig context and cluster to the chosen
// kubeconfig, pointing to the group.
func (g *Group) GetKubeconfig() (clientcmd.ClientConfig, error) {
	return g.Space.BuildKubeconfig(types.NamespacedName{Namespace: g.Name})
}

// GetKubeconfig upserts a controlplane context and cluster to the chosen kubeconfig.
func (ctp *ControlPlane) GetKubeconfig() (clientcmd.ClientConfig, error) {
	return ctp.Group.Space.BuildKubeconfig(ctp.NamespacedName())
}

func acceptState(s Accepting, navCtx *navContext) (msg string, err error) {
	config, err := s.GetKubeconfig()
	if err != nil {
		return "", err
	}

	raw, err := config.RawConfig()
	if err != nil {
		return "", err
	}

	if err := navCtx.contextWriter.Write(&raw); err != nil {
		return "", err
	}

	return fmt.Sprintf(contextSwitchedFmt, withUpboundPrefix(s.Breadcrumbs().styledString())), nil
}
