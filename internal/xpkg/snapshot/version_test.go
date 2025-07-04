// Copyright 2025 Upbound Inc.
// All rights reserved

package snapshot

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestVersionMatch(t *testing.T) {
	type args struct {
		constraint string
		versions   []string
	}
	type want struct {
		matched bool
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"MatchingVersionSupplied": {
			reason: "Should return a match when exact match found.",
			args: args{
				constraint: "v0.20.0",
				versions:   []string{"v0.20.0", "v0.20.1"},
			},
			want: want{
				matched: true,
			},
		},
		"MatchingConstraintSupplied": {
			reason: "Should return a match when matching constraint supplied.",
			args: args{
				constraint: ">=v0.19.0",
				versions:   []string{"v0.20.0"},
			},
			want: want{
				matched: true,
			},
		},
		"NonMatchingVersionSupplied": {
			reason: "Should not return a match when exact match not found.",
			args: args{
				constraint: "v0.21.0",
				versions:   []string{"v0.20.0", "v0.20.1"},
			},
			want: want{
				matched: false,
			},
		},
		"NonMatchingConstraintSupplied": {
			reason: "Should return a match when matching constraint supplied.",
			args: args{
				constraint: ">=v1.0.0",
				versions:   []string{"v0.20.0"},
			},
			want: want{
				matched: false,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			matched := versionMatch(tc.args.constraint, tc.args.versions)

			if diff := cmp.Diff(tc.want.matched, matched); diff != "" {
				t.Errorf("\n%s\nVersionMatch(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
