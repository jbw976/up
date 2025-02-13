// Copyright 2025 Upbound Inc.
// All rights reserved

package xpkg

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestTypes(t *testing.T) {
	type args struct {
		pkgType string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   bool
	}{
		"NotAPackageType": {
			reason: "We should return false when given an invalid package.",
			args: args{
				pkgType: "fake",
			},
			want: false,
		},
		"ConfigurationIsPackage": {
			reason: "We should return true when given a configuration package.",
			args: args{
				pkgType: "configuration",
			},
			want: true,
		},
		"ProviderIsPackage": {
			reason: "We should return true when given a provider package.",
			args: args{
				pkgType: "provider",
			},
			want: true,
		},
		"FunctionIsPackage": {
			reason: "We should return true when the given a function package.",
			args: args{
				pkgType: "function",
			},
			want: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p := Package(tc.args.pkgType)
			valid := p.IsValid()

			if diff := cmp.Diff(tc.want, valid); diff != "" {
				t.Errorf("\n%s\nIsValid(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
