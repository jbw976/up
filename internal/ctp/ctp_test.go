// Copyright 2025 Upbound Inc.
// All rights reserved

package ctp

import (
	"testing"

	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

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

func TestDetermineCrossplaneVersion(t *testing.T) {
	t.Parallel()

	const versions = `- 1.18.0-up.1
- 1.18.3-up.1
- 1.18.5-up.1
- 1.19.0-up.1
- 1.19.2-up.1
- 1.20.0-up.1
- 1.20.1-up.1
- 2.0.2-up.2
- 2.0.2-up.3
- 2.0.2-up.4
- 2.0.2-up.5
`

	tests := map[string]struct {
		cfg           *ensureDevControlPlaneConfig
		configMapData map[string]string
		expectError   bool
		expected      *spacesv1beta1.CrossplaneSpec
	}{
		"AlreadySet": {
			cfg: &ensureDevControlPlaneConfig{
				spacesConfig: spacesConfig{
					crossplane: &spacesv1beta1.CrossplaneSpec{Version: ptr.To("1.20.0")},
				},
			},
			expected: &spacesv1beta1.CrossplaneSpec{Version: ptr.To("1.20.0")},
		},
		"NoConstraint": {
			cfg: &ensureDevControlPlaneConfig{
				spacesConfig: spacesConfig{
					crossplaneVersionConstraint: "",
				},
			},
			expected: defaultCrossplaneSpec(),
		},
		"InvalidConstraint": {
			cfg: &ensureDevControlPlaneConfig{
				spacesConfig: spacesConfig{
					crossplaneVersionConstraint: "invalid-constraint",
				},
			},
			expectError: true,
		},
		"ConfigMapNotFound": {
			cfg: &ensureDevControlPlaneConfig{
				spacesConfig: spacesConfig{
					crossplaneVersionConstraint: ">=1.18.0",
				},
			},
			expectError: true,
		},
		"InvalidYAMLInConfigMap": {
			cfg: &ensureDevControlPlaneConfig{
				spacesConfig: spacesConfig{
					crossplaneVersionConstraint: ">=1.18.0",
				},
			},
			configMapData: map[string]string{
				"versions": "invalid yaml content",
			},
			expectError: true,
		},
		"ExactMatch": {
			cfg: &ensureDevControlPlaneConfig{
				spacesConfig: spacesConfig{
					crossplaneVersionConstraint: "1.18.3-up.1",
				},
			},
			configMapData: map[string]string{
				"versions": versions,
			},
			expected: &spacesv1beta1.CrossplaneSpec{
				Version: ptr.To("1.18.3-up.1"),
				AutoUpgradeSpec: &spacesv1beta1.CrossplaneAutoUpgradeSpec{
					Channel: ptr.To(spacesv1beta1.CrossplaneUpgradeNone),
				},
			},
		},
		"V1ConstraintMatch": {
			cfg: &ensureDevControlPlaneConfig{
				spacesConfig: spacesConfig{
					crossplaneVersionConstraint: "^v1.18.0-up.0",
				},
			},
			configMapData: map[string]string{
				"versions": versions,
			},
			expected: &spacesv1beta1.CrossplaneSpec{
				Version: ptr.To("1.20.1-up.1"),
				AutoUpgradeSpec: &spacesv1beta1.CrossplaneAutoUpgradeSpec{
					Channel: ptr.To(spacesv1beta1.CrossplaneUpgradeNone),
				},
			},
		},
		"V2ConstraintMatch": {
			cfg: &ensureDevControlPlaneConfig{
				spacesConfig: spacesConfig{
					crossplaneVersionConstraint: "^v2.0.0-up.0",
				},
			},
			configMapData: map[string]string{
				"versions": versions,
			},
			expected: &spacesv1beta1.CrossplaneSpec{
				Version: ptr.To("2.0.2-up.5"),
				AutoUpgradeSpec: &spacesv1beta1.CrossplaneAutoUpgradeSpec{
					Channel: ptr.To(spacesv1beta1.CrossplaneUpgradeNone),
				},
			},
		},
		"InvalidVersionInConfigMap": {
			cfg: &ensureDevControlPlaneConfig{
				spacesConfig: spacesConfig{
					crossplaneVersionConstraint: ">=1.18.0",
				},
			},
			configMapData: map[string]string{
				"versions": `- "1.17.5"
- "invalid-version"
- "1.19.1"
- "1.20.0"`,
			},
			expected: &spacesv1beta1.CrossplaneSpec{
				Version: ptr.To("1.20.0"),
				AutoUpgradeSpec: &spacesv1beta1.CrossplaneAutoUpgradeSpec{
					Channel: ptr.To(spacesv1beta1.CrossplaneUpgradeNone),
				},
			},
		},
		"MultipleConstraints": {
			cfg: &ensureDevControlPlaneConfig{
				spacesConfig: spacesConfig{
					crossplaneVersionConstraint: ">=1.18.0-up.0,<1.20.0-up.0",
				},
			},
			configMapData: map[string]string{
				"versions": versions,
			},
			expected: &spacesv1beta1.CrossplaneSpec{
				Version: ptr.To("1.19.2-up.1"),
				AutoUpgradeSpec: &spacesv1beta1.CrossplaneAutoUpgradeSpec{
					Channel: ptr.To(spacesv1beta1.CrossplaneUpgradeNone),
				},
			},
		},
		"NoMatchingVersion": {
			cfg: &ensureDevControlPlaneConfig{
				spacesConfig: spacesConfig{
					crossplaneVersionConstraint: ">=2.1.0-up.0",
				},
			},
			configMapData: map[string]string{
				"versions": versions,
			},
			expectError: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			scheme := runtime.NewScheme()
			_ = corev1.AddToScheme(scheme)

			var objects []runtime.Object
			if tc.configMapData != nil {
				objects = append(objects, &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "upbound-system",
						Name:      "crossplane-versions-public",
					},
					Data: tc.configMapData,
				})
			}
			fc := fake.NewFakeClient(objects...)

			got, err := determineCrossplaneVersion(t.Context(), fc, tc.cfg)

			if tc.expectError {
				assert.Assert(t, err != nil)
				return
			}

			assert.NilError(t, err)
			assert.DeepEqual(t, tc.expected, got)
		})
	}
}
