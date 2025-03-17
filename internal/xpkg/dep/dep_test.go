// Copyright 2025 Upbound Inc.
// All rights reserved

package dep

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/utils/ptr"

	"github.com/crossplane/crossplane/apis/pkg/v1beta1"

	"github.com/upbound/up/internal/xpkg/dep/resolver/image"
)

func TestNew(t *testing.T) {
	providerAws := "crossplane/provider-aws"
	functionTest := "crossplane-contrib/function-test"
	privateProviderAws := "hostname:8443/crossplane/provider-aws"

	type args struct {
		pkg string
		t   string
	}

	type want struct {
		dep v1beta1.Dependency
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"EmptyVersion": {
			args: args{
				pkg: providerAws,
				t:   "provider",
			},
			want: want{
				dep: v1beta1.Dependency{
					Package:     providerAws,
					Type:        ptr.To(v1beta1.ProviderPackageType),
					Constraints: image.DefaultVer,
				},
			},
		},
		"FunctionWithVersion": {
			args: args{
				pkg: functionTest,
				t:   "function",
			},
			want: want{
				dep: v1beta1.Dependency{
					Package:     functionTest,
					Type:        ptr.To(v1beta1.FunctionPackageType),
					Constraints: image.DefaultVer,
				},
			},
		},
		"VersionSuppliedAt": {
			args: args{
				pkg: fmt.Sprintf("%s@%s", providerAws, "v1.0.0"),
				t:   "provider",
			},
			want: want{
				dep: v1beta1.Dependency{
					Package:     providerAws,
					Type:        ptr.To(v1beta1.ProviderPackageType),
					Constraints: "v1.0.0",
				},
			},
		},
		"VersionConstraintSuppliedAt": {
			args: args{
				pkg: fmt.Sprintf("%s@%s", providerAws, ">=v1.0.0"),
				t:   "configuration",
			},
			want: want{
				dep: v1beta1.Dependency{
					Package:     providerAws,
					Type:        ptr.To(v1beta1.ConfigurationPackageType),
					Constraints: ">=v1.0.0",
				},
			},
		},
		"VersionSuppliedColon": {
			args: args{
				pkg: fmt.Sprintf("%s:%s", providerAws, "v1.0.0"),
				t:   "provider",
			},
			want: want{
				dep: v1beta1.Dependency{
					Package:     providerAws,
					Type:        ptr.To(v1beta1.ProviderPackageType),
					Constraints: "v1.0.0",
				},
			},
		},
		"VersionConstraintSuppliedColon": {
			args: args{
				pkg: fmt.Sprintf("%s:%s", providerAws, ">=v1.0.0"),
				t:   "configuration",
			},
			want: want{
				dep: v1beta1.Dependency{
					Package:     providerAws,
					Type:        ptr.To(v1beta1.ConfigurationPackageType),
					Constraints: ">=v1.0.0",
				},
			},
		},
		"PrivateRegistryAndVersionSuppliedColon": {
			args: args{
				pkg: fmt.Sprintf("%s:%s", privateProviderAws, ">=v1.0.0"),
				t:   "configuration",
			},
			want: want{
				dep: v1beta1.Dependency{
					Package:     privateProviderAws,
					Type:        ptr.To(v1beta1.ConfigurationPackageType),
					Constraints: ">=v1.0.0",
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			d := NewWithType(tc.args.pkg, tc.args.t)

			if diff := cmp.Diff(tc.want.dep, d); diff != "" {
				t.Errorf("\n%s\nNew(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}
