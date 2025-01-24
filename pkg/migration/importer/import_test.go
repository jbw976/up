// Copyright 2025 Upbound Inc.
// All rights reserved

package importer

import (
	"testing"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/google/go-cmp/cmp"
)

func Test_printConditions(t *testing.T) {
	type args struct {
		conditions []xpv1.ConditionType
	}
	type want struct {
		out string
	}
	cases := map[string]struct {
		args args
		want want
	}{
		"Empty": {
			args: args{
				conditions: []xpv1.ConditionType{},
			},
			want: want{
				out: "",
			},
		},
		"Single": {
			args: args{
				conditions: []xpv1.ConditionType{
					xpv1.TypeReady,
				},
			},
			want: want{
				out: "Ready",
			},
		},
		"Double": {
			args: args{
				conditions: []xpv1.ConditionType{
					xpv1.TypeReady,
					xpv1.TypeSynced,
				},
			},
			want: want{
				out: "Ready and Synced",
			},
		},
		"More": {
			args: args{
				conditions: []xpv1.ConditionType{
					xpv1.TypeReady,
					xpv1.TypeSynced,
					"Installed",
					"Healthy",
				},
			},
			want: want{
				out: "Ready, Synced, Installed, and Healthy",
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := printConditions(tc.args.conditions)
			if diff := cmp.Diff(got, tc.want.out); diff != "" {
				t.Errorf("printConditions() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
