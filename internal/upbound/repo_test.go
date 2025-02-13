// Copyright 2025 Upbound Inc.
// All rights reserved

package upbound

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseRepository(t *testing.T) {
	type args struct {
		repository      string
		defaultRegistry string
	}
	type want struct {
		registry string
		org      string
		repoName string
		err      error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ValidRepositoryWithRegistry": {
			reason: "We should parse a fully qualified repository with a registry, org, and repo correctly.",
			args: args{
				repository:      "docker.io/myorg/myrepo",
				defaultRegistry: "docker.io",
			},
			want: want{
				registry: "index.docker.io",
				org:      "myorg",
				repoName: "myrepo",
			},
		},
		"ValidRepositoryWithoutRegistry": {
			reason: "We should parse a repository without an explicit registry, using the default registry instead.",
			args: args{
				repository:      "myorg/myrepo",
				defaultRegistry: "docker.io",
			},
			want: want{
				registry: "index.docker.io",
				org:      "myorg",
				repoName: "myrepo",
			},
		},
		"RepositoryWithMissingOrg": {
			reason: "We should return an error if the repository string does not contain both org and repo.",
			args: args{
				repository:      "myrepo",
				defaultRegistry: "docker.io",
			},
			want: want{
				registry: "index.docker.io", // Docker assumes 'library' as the default org
				org:      "library",
				repoName: "myrepo",
			},
		},
		"OtherRepositoryWithMissingOrg": {
			reason: "We should return an error if the repository string does not contain both org and repo.",
			args: args{
				repository:      "myrepo",
				defaultRegistry: "registry.upbound.io",
			},
			want: want{
				err: fmt.Errorf("invalid repository format: %q, expected format 'org/repo' or 'registry/org/repo'", "myrepo"),
			},
		},

		"InvalidRepositoryFormat": {
			reason: "We should return an error if the repository is missing parts.",
			args: args{
				repository:      "registry.io/onlyonepart",
				defaultRegistry: "docker.io",
			},
			want: want{
				err: fmt.Errorf("invalid repository format: %q, expected format 'org/repo' or 'registry/org/repo'", "onlyonepart"),
			},
		},
		"ValidRepositoryWithoutDefaultRegistry": {
			reason: "We should use the registry from the repository if it is provided, even if no default registry is set.",
			args: args{
				repository:      "gcr.io/myorg/myrepo",
				defaultRegistry: "",
			},
			want: want{
				registry: "gcr.io",
				org:      "myorg",
				repoName: "myrepo",
			},
		},
		"ValidRepositoryWithCustomRegistry": {
			reason: "We should parse a repository using a custom default registry if the registry is not explicitly set in the repository.",
			args: args{
				repository:      "myorg/myrepo",
				defaultRegistry: "custom.registry.io",
			},
			want: want{
				registry: "custom.registry.io",
				org:      "myorg",
				repoName: "myrepo",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			registry, org, repoName, err := ParseRepository(tc.args.repository, tc.args.defaultRegistry)

			// Compare the errors using cmp
			if diff := cmp.Diff(tc.want.err, err, cmp.Comparer(func(x, y error) bool {
				return x != nil && y != nil && x.Error() == y.Error()
			})); diff != "" {
				t.Errorf("\n%s\nparseRepository(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			// Compare the individual parts of the result
			if diff := cmp.Diff(tc.want.registry, registry); diff != "" {
				t.Errorf("\n%s\nparseRepository(...): -want registry, +got registry:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.org, org); diff != "" {
				t.Errorf("\n%s\nparseRepository(...): -want org, +got org:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.repoName, repoName); diff != "" {
				t.Errorf("\n%s\nparseRepository(...): -want repoName, +got repoName:\n%s", tc.reason, diff)
			}
		})
	}
}
