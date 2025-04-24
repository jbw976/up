// Copyright 2025 Upbound Inc.
// All rights reserved

package xpkg

import (
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

const errInvalidPkgName = "invalid package dependency supplied"

// ValidDep validates a package dependency. Package dependencies must be
// fully-qualified image paths (i.e., include the registry), but may omit the
// version. A semver constraint or digest version may be provided, appended to
// the repository path with either @ or :.
func ValidDep(pkg string) (bool, error) {
	upkg := strings.ReplaceAll(pkg, "@", ":")
	err := parsePackageReference(upkg)
	return err == nil, errors.Wrap(err, errInvalidPkgName)
}

func parsePackageReference(pkg string) error {
	if pkg == "" {
		return errors.Errorf("empty package name")
	}

	var version string
	repository := pkg
	if strings.Contains(pkg, ":") {
		// Strip version constraint off to validate the repository.
		repository, version, _ = strings.Cut(pkg, ":")
	}

	// Validate the repository part.
	_, err := name.NewRepository(repository, name.StrictValidation)
	if err != nil {
		return errors.Wrap(err, "could not parse package repository")
	}

	// Validate the version constraint.

	if version == "" {
		// No version provided; we'll find the latest version later.
		return nil
	}

	if strings.Contains(version, ":") {
		// Validate as a digest.
		if _, err := v1.NewHash(version); err != nil {
			return errors.Wrap(err, "invalid digest version constraint")
		}
	} else {
		// Validate as a semver constraint.
		_, err := semver.NewConstraint(version)
		if err != nil {
			return errors.Wrap(err, "invalid SemVer constraint")
		}
	}

	return nil
}
