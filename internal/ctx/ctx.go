// Copyright 2024 Upbound Inc
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

// Package ctx contains ctx navigation functions
package ctx

import (
	"context"

	"k8s.io/client-go/tools/clientcmd"

	ctxcmd "github.com/upbound/up/cmd/up/ctx"
	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/upbound"
)

// GetCurrentSpaceNavigation derives the state of the current navigation using
// the same process as up ctx.
func GetCurrentSpaceNavigation(ctx context.Context, upCtx *upbound.Context) (ctxcmd.NavigationState, error) {
	po := clientcmd.NewDefaultPathOptions()
	var err error

	conf, err := po.GetStartingConfig()
	if err != nil {
		return nil, err
	}
	return ctxcmd.DeriveState(ctx, upCtx, conf, kube.GetIngressHost)
}
