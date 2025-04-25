// Copyright 2025 Upbound Inc.
// All rights reserved

// Package imageutil contains functions to work with all kind of images.
package imageutil

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	projectv1alpha1 "github.com/upbound/up/pkg/apis/project/v1alpha1"
)

func TestRewriteImage(t *testing.T) {
	type args struct {
		image   string
		configs []projectv1alpha1.ImageConfig
	}

	imageConfigs := []projectv1alpha1.ImageConfig{
		{
			MatchImages: []projectv1alpha1.ImageMatch{
				{
					Prefix: "xpkg.upbound.io",
					Type:   "Prefix",
				},
			},
			RewriteImage: projectv1alpha1.ImageRewrite{
				Prefix: "609897127049.dkr.ecr.eu-central-1.amazonaws.com",
			},
		},
		{
			MatchImages: []projectv1alpha1.ImageMatch{
				{
					Prefix: "xpkg.upbound.io/upbound/function-kcl-base:v0.11.2-up.1",
					Type:   "Prefix",
				},
			},
			RewriteImage: projectv1alpha1.ImageRewrite{
				Prefix: "docker.io/haarchri/function-kcl-base:v0.11.2-up.1",
			},
		},
	}

	cases := map[string]struct {
		reason string
		args   args
		want   string
	}{
		"ExactMatchSpecificWins": {
			reason: "Should match most specific prefix.",
			args: args{
				image:   "xpkg.upbound.io/upbound/function-kcl-base:v0.11.2-up.1",
				configs: imageConfigs,
			},
			want: "docker.io/haarchri/function-kcl-base:v0.11.2-up.1",
		},
		"GeneralPrefixMatch": {
			reason: "Should match general xpkg.upbound.io and rewrite.",
			args: args{
				image:   "xpkg.upbound.io/crossplane/provider-aws:v0.32.0",
				configs: imageConfigs,
			},
			want: "609897127049.dkr.ecr.eu-central-1.amazonaws.com/crossplane/provider-aws:v0.32.0",
		},
		"NoMatchReturnsOriginal": {
			reason: "Should return original if no prefix matches.",
			args: args{
				image:   "docker.io/library/nginx:latest",
				configs: imageConfigs,
			},
			want: "docker.io/library/nginx:latest",
		},
		"PartialMatchNotLongest": {
			reason: "Should match longest valid prefix .",
			args: args{
				image:   "xpkg.upbound.io/upbound/function-kcl-base:v0.11.2-up.1-extra",
				configs: imageConfigs,
			},
			want: "docker.io/haarchri/function-kcl-base:v0.11.2-up.1-extra",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := RewriteImage(tc.args.image, tc.args.configs)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nRewriteImage(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
