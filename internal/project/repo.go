// Copyright 2025 Upbound Inc.
// All rights reserved

package project

import (
	"strings"

	"github.com/google/go-containerregistry/pkg/name"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/pkg/apis/project/v1alpha1"
)

// DetermineRepository returns the repository to use when running a project
// based on the user's current configuration.
func DetermineRepository(upCtx *upbound.Context, proj *v1alpha1.Project, override string) (string, error) {
	defaultRegistry := upCtx.RegistryEndpoint.Host

	// If the user explicitly set the repository, use it, but if it's in the
	// Upbound registry check that the repository is owned by the org matching
	// their current profile.
	if override != "" {
		ref, err := name.NewRepository(override, name.WithDefaultRegistry(defaultRegistry))
		if err != nil {
			return "", errors.Wrap(err, "failed to parse repository")
		}

		if ref.RegistryStr() == defaultRegistry {
			_, org, _, err := upbound.ParseRepository(override, upCtx.RegistryEndpoint.Host)
			if err != nil {
				return "", err
			}

			if org != upCtx.Organization {
				return "", errors.New("specified repository does not belong to your current organization; use `up profile use` to select a different organization")
			}
		}

		// Make sure c.Repository is fully qualified.
		return ref.String(), nil
	}

	// If the user didn't explicitly set a repository, and the project's
	// repository is in the Upbound registry but not owned by the user's
	// organization, construct a new repository name that is owned by them. This
	// gives users the maximum chance of `up project run` Just Working when they
	// check out an example repo.

	ref, err := name.NewRepository(proj.Spec.Repository, name.WithDefaultRegistry(defaultRegistry))
	if err != nil {
		return "", errors.Wrap(err, "failed to parse repository")
	}

	if ref.RegistryStr() == defaultRegistry {
		_, _, repoName, err := upbound.ParseRepository(proj.Spec.Repository, upCtx.RegistryEndpoint.Host)
		if err != nil {
			return "", err
		}

		// Always use the host and org from the context
		return strings.Join([]string{upCtx.RegistryEndpoint.Host, upCtx.Organization, repoName}, "/"), nil
	}

	return proj.Spec.Repository, nil
}
