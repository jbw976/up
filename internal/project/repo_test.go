// Copyright 2025 Upbound Inc.
// All rights reserved

package project

import (
	"net/url"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/pkg/apis/project/v1alpha1"
)

func TestDetermineRepository(t *testing.T) {
	tests := map[string]struct {
		projectRepo string
		override    string

		want    string
		wantErr string
	}{
		"UpboundRegistryOverride": {
			projectRepo: "xpkg.upbound.io/example/project",
			override:    "xpkg.upbound.io/my-org/other-project",
			want:        "xpkg.upbound.io/my-org/other-project",
		},
		"UpboundRegistryOverrideWrongOrg": {
			projectRepo: "xpkg.upbound.io/example/project",
			override:    "xpkg.upbound.io/other-org/project",
			wantErr:     "repository does not belong to your current organization",
		},
		"OtherRegistryOverride": {
			projectRepo: "xpkg.upbound.io/example/project",
			override:    "my-registry.example.com/project",
			want:        "my-registry.example.com/project",
		},
		"UpboundRegistryNoOverride": {
			projectRepo: "xpkg.upbound.io/example/project",
			want:        "xpkg.upbound.io/my-org/project",
		},
		"OtherRegistryNoOverride": {
			projectRepo: "my-registry.example.com/project",
			want:        "my-registry.example.com/project",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			upCtx := &upbound.Context{
				RegistryEndpoint: &url.URL{
					Host: "xpkg.upbound.io",
				},
				Organization: "my-org",
			}

			proj := &v1alpha1.Project{
				Spec: &v1alpha1.ProjectSpec{
					Repository: tc.projectRepo,
				},
			}

			got, err := DetermineRepository(upCtx, proj, tc.override)
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
			} else {
				assert.Equal(t, tc.want, got)
			}
		})
	}
}
