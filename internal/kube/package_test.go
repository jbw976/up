// Copyright 2025 Upbound Inc.
// All rights reserved

package kube

import (
	"testing"

	"gotest.tools/v3/assert"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	xpkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
)

func TestPackageHasHealthyConditions(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		pkgrev xpkgv1.PackageRevision
		want   bool
	}{
		"ConfigurationV1NotReady": {
			pkgrev: &xpkgv1.ConfigurationRevision{
				Status: xpkgv1.PackageRevisionStatus{
					ConditionedStatus: *v1.NewConditionedStatus(),
				},
			},
			want: false,
		},
		"ConfigurationV1Ready": {
			pkgrev: &xpkgv1.ConfigurationRevision{
				Status: xpkgv1.PackageRevisionStatus{
					ConditionedStatus: *v1.NewConditionedStatus(xpkgv1.Healthy()),
				},
			},
			want: true,
		},
		"ConfigurationV2NotReady": {
			pkgrev: &xpkgv1.ConfigurationRevision{
				Status: xpkgv1.PackageRevisionStatus{
					ConditionedStatus: *v1.NewConditionedStatus(),
				},
			},
			want: false,
		},
		"ConfigurationV2Ready": {
			pkgrev: &xpkgv1.ConfigurationRevision{
				Status: xpkgv1.PackageRevisionStatus{
					ConditionedStatus: *v1.NewConditionedStatus(xpkgv1.RevisionHealthy()),
				},
			},
			want: true,
		},
		"FunctionV1NotReady": {
			pkgrev: &xpkgv1.FunctionRevision{
				Status: xpkgv1.FunctionRevisionStatus{
					PackageRevisionStatus: xpkgv1.PackageRevisionStatus{
						ConditionedStatus: *v1.NewConditionedStatus(),
					},
				},
			},
			want: false,
		},
		"FunctionV1Ready": {
			pkgrev: &xpkgv1.FunctionRevision{
				Status: xpkgv1.FunctionRevisionStatus{
					PackageRevisionStatus: xpkgv1.PackageRevisionStatus{
						ConditionedStatus: *v1.NewConditionedStatus(xpkgv1.Healthy()),
					},
				},
			},
			want: true,
		},
		"FunctionV2NotReady": {
			pkgrev: &xpkgv1.FunctionRevision{
				Status: xpkgv1.FunctionRevisionStatus{
					PackageRevisionStatus: xpkgv1.PackageRevisionStatus{
						ConditionedStatus: *v1.NewConditionedStatus(),
					},
				},
			},
			want: false,
		},
		"FunctionV2RevisionNotReady": {
			pkgrev: &xpkgv1.FunctionRevision{
				Status: xpkgv1.FunctionRevisionStatus{
					PackageRevisionStatus: xpkgv1.PackageRevisionStatus{
						ConditionedStatus: *v1.NewConditionedStatus(xpkgv1.RuntimeHealthy()),
					},
				},
			},
			want: false,
		},
		"FunctionV2RuntimeNotReady": {
			pkgrev: &xpkgv1.FunctionRevision{
				Status: xpkgv1.FunctionRevisionStatus{
					PackageRevisionStatus: xpkgv1.PackageRevisionStatus{
						ConditionedStatus: *v1.NewConditionedStatus(xpkgv1.RevisionHealthy()),
					},
				},
			},
			want: false,
		},
		"FunctionV2Ready": {
			pkgrev: &xpkgv1.FunctionRevision{
				Status: xpkgv1.FunctionRevisionStatus{
					PackageRevisionStatus: xpkgv1.PackageRevisionStatus{
						ConditionedStatus: *v1.NewConditionedStatus(xpkgv1.RevisionHealthy(), xpkgv1.RuntimeHealthy()),
					},
				},
			},
			want: true,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := packageHasHealthyConditions(tc.pkgrev)
			assert.Equal(t, got, tc.want)
		})
	}
}
