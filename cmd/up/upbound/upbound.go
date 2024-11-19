// Copyright 2021 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Please note: As of March 2023, the `upbound` commands have been disabled.
// We're keeping the code here for now, so they're easily resurrected.
// The upbound commands were meant to support the Upbound self-hosted option.

package upbound

import (
	"github.com/alecthomas/kong"

	"github.com/upbound/up/internal/feature"
	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/kube"
)

// BeforeReset is the first hook to run.
func (c *Cmd) BeforeReset(p *kong.Path, maturity feature.Maturity) error {
	return feature.HideMaturity(p, maturity)
}

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *Cmd) AfterApply(kongCtx *kong.Context) error {
	kubeconfig, err := kube.GetKubeConfig(c.Kubeconfig)
	if err != nil {
		return err
	}

	kongCtx.Bind(&install.Context{
		Kubeconfig: kubeconfig,
		Namespace:  c.Namespace,
	})
	return nil
}

// Cmd contains commands for managing Upbound.
type Cmd struct {
	Kubeconfig string `help:"Override default kubeconfig path." type:"existingfile"`
	Namespace  string `default:"upbound-system"                 env:"UPBOUND_NAMESPACE" help:"Kubernetes namespace for Upbound." short:"n"`
}
