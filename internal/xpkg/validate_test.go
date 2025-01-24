// Copyright 2025 Upbound Inc.
// All rights reserved

package xpkg

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

func TestValidVer(t *testing.T) {
	type args struct {
		pkg string
	}

	type want struct {
		valid bool
		err   error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ErrEmptyPackage": {
			reason: "Should return an error that an empty package is invalid.",
			args:   args{},
			want: want{
				err: errors.New("could not parse reference: empty package name, invalid package dependency supplied"),
			},
		},
		"SuccessNoVersion": {
			reason: "Should return that the package name is valid.",
			args: args{
				pkg: "crossplane/provider-aws",
			},
			want: want{
				valid: true,
			},
		},
		"SuccessVersionSpecifiedWithAt": {
			reason: "Should return that the package name is valid with version specified using '@'.",
			args: args{
				pkg: "crossplane/provider-aws@v1.2.0",
			},
			want: want{
				valid: true,
			},
		},
		"SuccessSemVersionSpecifiedWithAt": {
			reason: "Should return that the package name is valid with version specified using '@'.",
			args: args{
				pkg: "crossplane/provider-aws@>=v1.2.0",
			},
			want: want{
				valid: true,
			},
		},
		"SuccessSemVersionSpecifiedWithColon": {
			reason: "Should return that the package name is valid with version specified using ':'.",
			args: args{
				pkg: "crossplane/provider-aws:>=v1.2.0",
			},
			want: want{
				valid: true,
			},
		},
		"InvalidPackageName": {
			reason: "Should return an error if the package name is invalid.",
			args: args{
				pkg: "invalid-package-name!@1.0.0",
			},
			want: want{
				err: errors.New("invalid package dependency supplied: could not parse reference: invalid-package-name!"),
			},
		},
		"InvalidSemVerConstraint": {
			reason: "Should return an error if the version constraint is invalid.",
			args: args{
				pkg: "crossplane/provider-aws@invalid-version",
			},
			want: want{
				err: errors.New("invalid SemVer constraint invalid-version: invalid package dependency supplied"),
			},
		},
		"SuccessLatestColonIsDelimiter": {
			reason: "Should correctly identify the latest colon as the version delimiter.",
			args: args{
				pkg: "registry/repo/crossplane/provider-aws:>=v1.2.0",
			},
			want: want{
				valid: true,
			},
		},
		"ErrorLatestColonIsDelimiter": {
			reason: "Should correctly identify the latest colon as the version delimiter.",
			args: args{
				pkg: "registry/repo/crossplane/provider-aws:>=v1.2:0",
			},
			want: want{
				err: errors.New("invalid SemVer constraint >=v1.2:0: invalid package dependency supplied"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			valid, err := parsePackageReference(tc.args.pkg)

			if diff := cmp.Diff(tc.want.valid, valid); diff != "" {
				t.Errorf("\n%s\nparsePackageReference(...): -want valid, +got valid:\n%s", tc.reason, diff)
			}

			if tc.want.err != nil {
				if err == nil || !errorsContains(err, tc.want.err.Error()) {
					t.Errorf("\n%s\nparsePackageReference(...): expected error containing %q, got %v", tc.reason, tc.want.err.Error(), err)
				}
			} else if err != nil {
				t.Errorf("\n%s\nparsePackageReference(...): expected no error, got %v", tc.reason, err)
			}
		})
	}
}

// helper function to check if the error chain contains a substring of an error message.
func errorsContains(err error, msg string) bool {
	for err != nil {
		if err.Error() == msg {
			return true
		}
		err = errors.Unwrap(err)
	}
	return false
}
