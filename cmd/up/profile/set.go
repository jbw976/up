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

package profile

import (
	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/upbound"
)

type setCmd struct {
	Key   string `arg:"" enum:"organization,domain"             help:"The configuration key to set." required:""`
	Value string `arg:"" help:"The configuration value to set." required:""`
}

func (c *setCmd) Run(upCtx *upbound.Context) error {
	switch c.Key {
	case "organization":
		upCtx.Profile.Organization = c.Value
		// Clear the deprecated account field if organization is set.
		upCtx.Profile.Account = "" //nolint:staticcheck // Migration off deprecated value.

	case "domain":
		upCtx.Profile.Domain = c.Value

	default:
		// Should never hit this due to kong validation.
		return errors.New("invalid key")
	}

	if err := upCtx.Cfg.AddOrUpdateUpboundProfile(upCtx.ProfileName, upCtx.Profile); err != nil {
		return err
	}

	return errors.Wrap(upCtx.CfgSrc.UpdateConfig(upCtx.Cfg), errUpdateProfile)
}
