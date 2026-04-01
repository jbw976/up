// Copyright 2025 Upbound Inc.
// All rights reserved

package oci

import (
	"testing"

	"github.com/alecthomas/assert/v2"
	"helm.sh/helm/v3/pkg/chart"
)

func TestParseUxpV2RuntimeTags(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		loadedChart *chart.Chart
		wantCX      string
		wantCM      string
		wantErr     string
	}{
		{
			name: "ExplicitTags",
			loadedChart: &chart.Chart{
				Metadata: &chart.Metadata{AppVersion: "2.2.0-up.3"},
				Values: map[string]interface{}{
					"image": map[string]interface{}{
						"tag": "v2.2.0-up.1",
					},
					"upbound": map[string]interface{}{
						"manager": map[string]interface{}{
							"image": map[string]interface{}{
								"tag": "v2.2.0-up.3",
							},
						},
					},
				},
			},
			wantCX: "v2.2.0-up.1",
			wantCM: "v2.2.0-up.3",
		},
		{
			name: "FallbackToAppVersion",
			loadedChart: &chart.Chart{
				Metadata: &chart.Metadata{AppVersion: "2.1.4-up.2"},
				Values: map[string]interface{}{
					"image": map[string]interface{}{
						"tag": "",
					},
					"upbound": map[string]interface{}{
						"manager": map[string]interface{}{
							"image": map[string]interface{}{
								"tag": "",
							},
						},
					},
				},
			},
			wantCX: "v2.1.4-up.2",
			wantCM: "v2.1.4-up.2",
		},
		{
			name: "AppVersionWithoutVPrefix",
			loadedChart: &chart.Chart{
				Metadata: &chart.Metadata{AppVersion: "2.0.0-up.1"},
				Values: map[string]interface{}{
					"image":   map[string]interface{}{},
					"upbound": map[string]interface{}{},
				},
			},
			wantCX: "v2.0.0-up.1",
			wantCM: "v2.0.0-up.1",
		},
		{
			name: "SameMajorMinorPatchUsesAppVersionForControllerManager",
			loadedChart: &chart.Chart{
				Metadata: &chart.Metadata{AppVersion: "2.2.0-up.3"},
				Values: map[string]interface{}{
					"image": map[string]interface{}{
						"tag": "v2.2.0-up.1",
					},
					"upbound": map[string]interface{}{
						"manager": map[string]interface{}{
							"image": map[string]interface{}{
								"tag": "",
							},
						},
					},
				},
			},
			wantCX: "v2.2.0-up.1",
			wantCM: "v2.2.0-up.3",
		},
		{
			name: "DifferentCoreUsesCrossplaneTagForControllerManager",
			loadedChart: &chart.Chart{
				Metadata: &chart.Metadata{AppVersion: "2.0.1-up.2"},
				Values: map[string]interface{}{
					"image": map[string]interface{}{
						"tag": "v2.1.0-up.1",
					},
					"upbound": map[string]interface{}{
						"manager": map[string]interface{}{
							"image": map[string]interface{}{
								"tag": "",
							},
						},
					},
				},
			},
			wantCX: "v2.1.0-up.1",
			wantCM: "v2.1.0-up.1",
		},
		{
			name: "MissingAppVersion",
			loadedChart: &chart.Chart{
				Metadata: &chart.Metadata{},
				Values: map[string]interface{}{
					"image": map[string]interface{}{
						"tag": "",
					},
					"upbound": map[string]interface{}{
						"manager": map[string]interface{}{
							"image": map[string]interface{}{
								"tag": "",
							},
						},
					},
				},
			},
			wantErr: "failed to resolve crossplane image tag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cx, cm, err := parseUxpV2RuntimeTags("xpkg.upbound.io/spaces-artifacts/crossplane", "2.2.0-up.3", tt.loadedChart)
			if tt.wantErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.wantCX, cx)
			assert.Equal(t, tt.wantCM, cm)
		})
	}
}

func TestNormalizeMirrorTag(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", normalizeMirrorTag(""))
	assert.Equal(t, "v1.2.3", normalizeMirrorTag("1.2.3"))
	assert.Equal(t, "v2.2.0-up.1", normalizeMirrorTag("v2.2.0-up.1"))
}
