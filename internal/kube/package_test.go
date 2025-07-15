// Copyright 2025 Upbound Inc.
// All rights reserved

package kube

import (
	"testing"

	"gotest.tools/v3/assert"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	xpkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	xpkgv1beta1 "github.com/crossplane/crossplane/apis/pkg/v1beta1"
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

func TestLookupLockPackage(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		lockPackages []xpkgv1beta1.LockPackage
		source       string
		constraint   string

		wantFound bool
		wantPkg   xpkgv1beta1.LockPackage
	}{
		"EmptyLock": {
			source:     "xpkg.upbound.io/upbound/provider-aws-s3",
			constraint: "v1.23.1",
			wantFound:  false,
		},
		"NotFoundExactVersion": {
			lockPackages: []xpkgv1beta1.LockPackage{
				{
					Name:    "upbound-provider-aws-s3-abcdef",
					Source:  "xpkg.upbound.io/upbound/provider-aws-s3",
					Version: "v1.22.1",
				},
				{
					Name:    "upbound-provider-aws-s3-fedcba",
					Source:  "xpkg.upbound.io/upbound/provider-aws-ec2",
					Version: "fbb2f27ed8e365191664050cd9d09a9c09145700",
				},
			},
			source:     "xpkg.upbound.io/upbound/provider-aws-s3",
			constraint: "v1.23.1",
			wantFound:  false,
		},
		"NotFoundConstraint": {
			lockPackages: []xpkgv1beta1.LockPackage{
				{
					Name:    "upbound-provider-aws-s3-abcdef",
					Source:  "xpkg.upbound.io/upbound/provider-aws-s3",
					Version: "v1.22.1",
				},
				{
					Name:    "upbound-provider-aws-s3-fedcba",
					Source:  "xpkg.upbound.io/upbound/provider-aws-ec2",
					Version: "fbb2f27ed8e365191664050cd9d09a9c09145700",
				},
			},
			source:     "xpkg.upbound.io/upbound/provider-aws-s3",
			constraint: ">=v1.23.1",
			wantFound:  false,
		},
		"NotFoundDigest": {
			lockPackages: []xpkgv1beta1.LockPackage{
				{
					Name:    "upbound-provider-aws-s3-abcdef",
					Source:  "xpkg.upbound.io/upbound/provider-aws-s3",
					Version: "v1.22.1",
				},
				{
					Name:    "upbound-provider-aws-s3-fedcba",
					Source:  "xpkg.upbound.io/upbound/provider-aws-ec2",
					Version: "fbb2f27ed8e365191664050cd9d09a9c09145700",
				},
			},
			source:     "xpkg.upbound.io/upbound/provider-aws-ec2",
			constraint: "af6f4042403a1cd4b0fe825e0602e869b346ab00",
			wantFound:  false,
		},
		"FoundExactVersion": {
			lockPackages: []xpkgv1beta1.LockPackage{
				{
					Name:    "upbound-provider-aws-s3-abcdef",
					Source:  "xpkg.upbound.io/upbound/provider-aws-s3",
					Version: "v1.22.1",
				},
				{
					Name:    "upbound-provider-aws-s3-fedcba",
					Source:  "xpkg.upbound.io/upbound/provider-aws-ec2",
					Version: "fbb2f27ed8e365191664050cd9d09a9c09145700",
				},
			},
			source:     "xpkg.upbound.io/upbound/provider-aws-s3",
			constraint: "v1.22.1",
			wantFound:  true,
			wantPkg: xpkgv1beta1.LockPackage{
				Name:    "upbound-provider-aws-s3-abcdef",
				Source:  "xpkg.upbound.io/upbound/provider-aws-s3",
				Version: "v1.22.1",
			},
		},
		"FoundConstraint": {
			lockPackages: []xpkgv1beta1.LockPackage{
				{
					Name:    "upbound-provider-aws-s3-abcdef",
					Source:  "xpkg.upbound.io/upbound/provider-aws-s3",
					Version: "v1.22.1",
				},
				{
					Name:    "upbound-provider-aws-s3-fedcba",
					Source:  "xpkg.upbound.io/upbound/provider-aws-ec2",
					Version: "fbb2f27ed8e365191664050cd9d09a9c09145700",
				},
			},
			source:     "xpkg.upbound.io/upbound/provider-aws-s3",
			constraint: ">=v1.22.0",
			wantFound:  true,
			wantPkg: xpkgv1beta1.LockPackage{
				Name:    "upbound-provider-aws-s3-abcdef",
				Source:  "xpkg.upbound.io/upbound/provider-aws-s3",
				Version: "v1.22.1",
			},
		},
		"FoundDigest": {
			lockPackages: []xpkgv1beta1.LockPackage{
				{
					Name:    "upbound-provider-aws-s3-abcdef",
					Source:  "xpkg.upbound.io/upbound/provider-aws-s3",
					Version: "v1.22.1",
				},
				{
					Name:    "upbound-provider-aws-s3-fedcba",
					Source:  "xpkg.upbound.io/upbound/provider-aws-ec2",
					Version: "fbb2f27ed8e365191664050cd9d09a9c09145700",
				},
			},
			source:     "xpkg.upbound.io/upbound/provider-aws-ec2",
			constraint: "fbb2f27ed8e365191664050cd9d09a9c09145700",
			wantFound:  true,
			wantPkg: xpkgv1beta1.LockPackage{
				Name:    "upbound-provider-aws-s3-fedcba",
				Source:  "xpkg.upbound.io/upbound/provider-aws-ec2",
				Version: "fbb2f27ed8e365191664050cd9d09a9c09145700",
			},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			gotPkg, gotFound := lookupLockPackage(tc.lockPackages, tc.source, tc.constraint)
			assert.Equal(t, tc.wantFound, gotFound)
			assert.DeepEqual(t, tc.wantPkg, gotPkg)
		})
	}
}
