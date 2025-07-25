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
				err: errors.New("invalid package dependency supplied: empty package name"),
			},
		},
		"SuccessNoVersion": {
			reason: "Should return that the package name is valid.",
			args: args{
				pkg: "registry.example.com/crossplane/provider-aws",
			},
			want: want{
				valid: true,
			},
		},
		"SuccessVersionSpecifiedWithAt": {
			reason: "Should return that the package name is valid with version specified using '@'.",
			args: args{
				pkg: "registry.example.com/crossplane/provider-aws@v1.2.0",
			},
			want: want{
				valid: true,
			},
		},
		"SuccessSemVersionSpecifiedWithAt": {
			reason: "Should return that the package name is valid with version specified using '@'.",
			args: args{
				pkg: "registry.example.com/crossplane/provider-aws@>=v1.2.0",
			},
			want: want{
				valid: true,
			},
		},
		"SuccessSemVersionSpecifiedWithColon": {
			reason: "Should return that the package name is valid with version specified using ':'.",
			args: args{
				pkg: "registry.example.com/crossplane/provider-aws:>=v1.2.0",
			},
			want: want{
				valid: true,
			},
		},
		"SuccessDigestConstraint": {
			reason: "Should return valid for valid digest constraints.",
			args: args{
				pkg: "registry.example.com/crossplane/provider-aws@sha256:5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
			},
			want: want{
				valid: true,
			},
		},
		"SuccessDigestConstraintWithColon": {
			reason: "Should return valid for valid digest constraints using the colon delimiter.",
			args: args{
				pkg: "registry.example.com/crossplane/provider-aws:sha256:5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
			},
			want: want{
				valid: true,
			},
		},
		"InvalidDigestConstraint": {
			reason: "Should return valid for valid digest constraints.",
			args: args{
				pkg: "registry.example.com/crossplane/provider-aws@fakealgorithm:asdf",
			},
			want: want{
				err: errors.New("invalid package dependency supplied: invalid digest version constraint: found non-hex character in hash: s"),
			},
		},
		"InvalidPackageName": {
			reason: "Should return an error if the package name is invalid.",
			args: args{
				pkg: "registry.example.com/invalid-package-name!@1.0.0",
			},
			want: want{
				err: errors.New("invalid package dependency supplied: could not parse package repository: repository can only contain the characters `abcdefghijklmnopqrstuvwxyz0123456789_-./`: invalid-package-name!"), //nolint:revive // Error includes punctuation from invalid string.
			},
		},
		"IncompletePackageName": {
			reason: "Should return an error if the package name does not include a registry.",
			args: args{
				pkg: "crossplane/provider-aws@1.0.0",
			},
			want: want{
				err: errors.New("invalid package dependency supplied: could not parse package repository: strict validation requires the registry to be explicitly defined"),
			},
		},
		"InvalidSemVerConstraint": {
			reason: "Should return an error if the version constraint is invalid.",
			args: args{
				pkg: "registry.example.com/crossplane/provider-aws@invalid-version",
			},
			want: want{
				err: errors.New("invalid package dependency supplied: invalid SemVer constraint: improper constraint: invalid-version"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			valid, err := ValidDep(tc.args.pkg)

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
