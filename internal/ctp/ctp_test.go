// Copyright 2025 Upbound Inc.
// All rights reserved

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
