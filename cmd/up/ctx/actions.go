// Copyright 2024 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ctx

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	upboundPreviousContextSuffix = "-previous"
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
