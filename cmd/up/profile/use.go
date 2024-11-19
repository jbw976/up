// Copyright 2022 Upbound Inc
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

package profile

import (
	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/upbound"
)

const (
	errUpdateProfile = "unable to update profile"
)

type useCmd struct {
	Name string `arg:"" help:"Name of the Profile to use." predictor:"profiles" required:""`
}

// Run executes the Use command.
func (c *useCmd) Run(upCtx *upbound.Context) error {
	if err := upCtx.Cfg.SetDefaultUpboundProfile(c.Name); err != nil {
		return err
	}

	return errors.Wrap(upCtx.CfgSrc.UpdateConfig(upCtx.Cfg), errUpdateProfile)
}
