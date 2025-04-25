// Copyright 2025 Upbound Inc.
// All rights reserved

package image

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
)

func TestResolveTag(t *testing.T) {
	type args struct {
		dep     v1beta1.Dependency
		fetcher Fetcher
	}

	type want struct {
		tag string
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SuccessTagFound": {
			reason: "Should return tag.",
			args: args{
				dep: v1beta1.Dependency{
					Package:     "crossplane/provider-aws",
					Constraints: ">=v0.1.1",
				},
				fetcher: NewMockFetcher(
					WithTags(
						[]string{
							"v0.2.0",
							"alpha",
						},
					),
				),
			},
			want: want{
				tag: "v0.2.0",
			},
		},
		"SuccessNoVersionSupplied": {
			reason: "Should return tag.",
			args: args{
				dep: v1beta1.Dependency{
					Package:     "crossplane/provider-aws",
					Constraints: "",
				},
				fetcher: NewMockFetcher(
					WithTags(
						[]string{
							"v0.2.0",
							"alpha",
						},
					),
				),
			},
			want: want{
				tag: "v0.2.0",
			},
		},
		"ErrorInvalidTag": {
			reason: "Should return an error if dep has invalid constraint.",
			args: args{
				dep: v1beta1.Dependency{
					Package:     "crossplane/provider-aws",
					Constraints: "alpha",
				},
				fetcher: NewMockFetcher(
					WithError(
						errors.New(errInvalidConstraint),
					),
				),
			},
			want: want{
				err: errors.Wrap(errors.New("improper constraint: alpha"), errInvalidConstraint),
			},
		},
		"ErrorInvalidReference": {
			reason: "Should return an error if dep has invalid provider.",
			args: args{
				dep: v1beta1.Dependency{
					Package:     "",
					Constraints: "v1.0.0",
				},
				fetcher: NewMockFetcher(
					WithError(
						errors.New(errInvalidProviderRef),
					),
				),
			},
			want: want{
				err: errors.Wrapf(
					errors.Wrap(errors.New("could not parse reference: "), errInvalidProviderRef),
					"failed to fetch digest %s", "",
				),
			},
		},
		"ErrorFailedToFetchTags": {
			reason: "Should return an error if we could not fetch tags.",
			args: args{
				dep: v1beta1.Dependency{
					Package:     "crossplane/provider-aws",
					Constraints: ">=v1.0.0",
				},
				fetcher: NewMockFetcher(
					WithError(
						errors.New(errFailedToFetchTags),
					),
				),
			},
			want: want{
				err: errors.Wrap(errors.New(errFailedToFetchTags), errFailedToFetchTags),
			},
		},
		"NoMatchingVersionShowLatestVersions": {
			reason: "Should return an error with the latest available versions when no matching version is found.",
			args: args{
				dep: v1beta1.Dependency{
					Package:     "crossplane/provider-aws",
					Constraints: ">=v3.0.0",
				},
				fetcher: NewMockFetcher(
					WithTags(
						[]string{
							"v0.1.0",
							"v1.0.0",
							"v2.0.0",
						},
					),
				),
			},
			want: want{
				err: errors.New("supplied version does not match an existing version. Latest available versions: [v0.1.0 v1.0.0 v2.0.0]"),
			},
		},
		"NoMatchingVersionShowLatestVersionsMultiple": {
			reason: "Should return an error with the latest available versions when no matching version is found.",
			args: args{
				dep: v1beta1.Dependency{
					Package:     "crossplane/provider-aws",
					Constraints: ">=v4.0.0",
				},
				fetcher: NewMockFetcher(
					WithTags(
						[]string{
							"v0.1.0",
							"v1.0.0",
							"v2.0.0",
							"v3.0.0",
						},
					),
				),
			},
			want: want{
				err: errors.New("supplied version does not match an existing version. Latest available versions: [v1.0.0 v2.0.0 v3.0.0]"),
			},
		},
		"NoMatchingVersionShowFewerThanThreeVersions": {
			reason: "Should return an error with the available versions when fewer than three versions are available.",
			args: args{
				dep: v1beta1.Dependency{
					Package:     "crossplane/provider-aws",
					Constraints: ">=v3.0.0",
				},
				fetcher: NewMockFetcher(
					WithTags(
						[]string{
							"v1.0.0",
							"v2.0.0",
						},
					),
				),
			},
			want: want{
				err: errors.New("supplied version does not match an existing version. Latest available versions: [v1.0.0 v2.0.0]"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewResolver(WithFetcher(tc.args.fetcher))

			got, err := r.ResolveTag(context.Background(), tc.args.dep)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nResolveTag(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.tag, got); diff != "" {
				t.Errorf("\n%s\nResolveTag(...): -want tag, +got tag:\n%s", tc.reason, diff)
			}
		})
	}
}
