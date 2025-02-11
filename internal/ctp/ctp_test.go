// Copyright 2025 Upbound Inc
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

package ctp

import (
	"testing"

	"gotest.tools/v3/assert"
	"k8s.io/utils/ptr"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
)

func TestMatchesCrossplaneSpec(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		existing spacesv1beta1.CrossplaneSpec
		desired  spacesv1beta1.CrossplaneSpec
		want     bool
	}{
		"EqualEmpty": {
			existing: spacesv1beta1.CrossplaneSpec{},
			desired:  spacesv1beta1.CrossplaneSpec{},
			want:     true,
		},
		"EqualNoVersions": {
			existing: spacesv1beta1.CrossplaneSpec{
				AutoUpgradeSpec: &spacesv1beta1.CrossplaneAutoUpgradeSpec{
					Channel: ptr.To(spacesv1beta1.CrossplaneUpgradeRapid),
				},
			},
			desired: spacesv1beta1.CrossplaneSpec{
				AutoUpgradeSpec: &spacesv1beta1.CrossplaneAutoUpgradeSpec{
					Channel: ptr.To(spacesv1beta1.CrossplaneUpgradeRapid),
				},
			},
			want: true,
		},
		"DifferentChannels": {
			existing: spacesv1beta1.CrossplaneSpec{
				AutoUpgradeSpec: &spacesv1beta1.CrossplaneAutoUpgradeSpec{
					Channel: ptr.To(spacesv1beta1.CrossplaneUpgradeNone),
				},
			},
			desired: spacesv1beta1.CrossplaneSpec{
				AutoUpgradeSpec: &spacesv1beta1.CrossplaneAutoUpgradeSpec{
					Channel: ptr.To(spacesv1beta1.CrossplaneUpgradeRapid),
				},
			},
			want: false,
		},
		"EqualVersions": {
			existing: spacesv1beta1.CrossplaneSpec{
				AutoUpgradeSpec: &spacesv1beta1.CrossplaneAutoUpgradeSpec{
					Channel: ptr.To(spacesv1beta1.CrossplaneUpgradeNone),
				},
				Version: ptr.To("1.18.0"),
			},
			desired: spacesv1beta1.CrossplaneSpec{
				AutoUpgradeSpec: &spacesv1beta1.CrossplaneAutoUpgradeSpec{
					Channel: ptr.To(spacesv1beta1.CrossplaneUpgradeNone),
				},
				Version: ptr.To("1.18.0"),
			},
			want: true,
		},
		"DifferentVersions": {
			existing: spacesv1beta1.CrossplaneSpec{
				AutoUpgradeSpec: &spacesv1beta1.CrossplaneAutoUpgradeSpec{
					Channel: ptr.To(spacesv1beta1.CrossplaneUpgradeNone),
				},
				Version: ptr.To("1.18.0"),
			},
			desired: spacesv1beta1.CrossplaneSpec{
				AutoUpgradeSpec: &spacesv1beta1.CrossplaneAutoUpgradeSpec{
					Channel: ptr.To(spacesv1beta1.CrossplaneUpgradeNone),
				},
				Version: ptr.To("1.19.0"),
			},
			want: false,
		},
		"EqualNoDesiredVersion": {
			existing: spacesv1beta1.CrossplaneSpec{
				AutoUpgradeSpec: &spacesv1beta1.CrossplaneAutoUpgradeSpec{
					Channel: ptr.To(spacesv1beta1.CrossplaneUpgradeRapid),
				},
				Version: ptr.To("1.19.0"),
			},
			desired: spacesv1beta1.CrossplaneSpec{
				AutoUpgradeSpec: &spacesv1beta1.CrossplaneAutoUpgradeSpec{
					Channel: ptr.To(spacesv1beta1.CrossplaneUpgradeRapid),
				},
			},
			want: true,
		},
		"EqualStates": {
			existing: spacesv1beta1.CrossplaneSpec{
				AutoUpgradeSpec: &spacesv1beta1.CrossplaneAutoUpgradeSpec{
					Channel: ptr.To(spacesv1beta1.CrossplaneUpgradeRapid),
				},
				State: ptr.To(spacesv1beta1.CrossplaneStateRunning),
			},
			desired: spacesv1beta1.CrossplaneSpec{
				AutoUpgradeSpec: &spacesv1beta1.CrossplaneAutoUpgradeSpec{
					Channel: ptr.To(spacesv1beta1.CrossplaneUpgradeRapid),
				},
				State: ptr.To(spacesv1beta1.CrossplaneStateRunning),
			},
			want: true,
		},
		"EqualNoDesiredState": {
			existing: spacesv1beta1.CrossplaneSpec{
				AutoUpgradeSpec: &spacesv1beta1.CrossplaneAutoUpgradeSpec{
					Channel: ptr.To(spacesv1beta1.CrossplaneUpgradeRapid),
				},
				State: ptr.To(spacesv1beta1.CrossplaneStateRunning),
			},
			desired: spacesv1beta1.CrossplaneSpec{
				AutoUpgradeSpec: &spacesv1beta1.CrossplaneAutoUpgradeSpec{
					Channel: ptr.To(spacesv1beta1.CrossplaneUpgradeRapid),
				},
			},
			want: true,
		},
		"DifferentStates": {
			existing: spacesv1beta1.CrossplaneSpec{
				AutoUpgradeSpec: &spacesv1beta1.CrossplaneAutoUpgradeSpec{
					Channel: ptr.To(spacesv1beta1.CrossplaneUpgradeRapid),
				},
				State: ptr.To(spacesv1beta1.CrossplaneStatePaused),
			},
			desired: spacesv1beta1.CrossplaneSpec{
				AutoUpgradeSpec: &spacesv1beta1.CrossplaneAutoUpgradeSpec{
					Channel: ptr.To(spacesv1beta1.CrossplaneUpgradeRapid),
				},
				State: ptr.To(spacesv1beta1.CrossplaneStateRunning),
			},
			want: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := matchesCrossplaneSpec(tc.existing, tc.desired)
			assert.Equal(t, got, tc.want)
		})
	}
}
