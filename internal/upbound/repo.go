// Copyright 2025 Upbound Inc.
// All rights reserved

package upbound

import (
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// ParseRepository parse a repository and normalize it.
func ParseRepository(repository string, defaultRegistry string) (registry, org, repoName string, err error) {
	ref, err := name.NewRepository(repository, name.WithDefaultRegistry(defaultRegistry))
	if err != nil {
		return "", "", "", errors.Wrap(err, "failed to parse repository")
	}
	reg := ref.Registry.String()
	repo := ref.RepositoryStr()
	repoParts := strings.SplitN(repo, "/", 2)

	// Ensure that repoParts contains at least two elements
	if len(repoParts) < 2 {
		return "", "", "", fmt.Errorf("invalid repository format: %q, expected format 'org/repo' or 'registry/org/repo'", repo)
	}

	return reg, repoParts[0], repoParts[1], nil
}
