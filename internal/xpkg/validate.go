// Copyright 2025 Upbound Inc.
// All rights reserved

package xpkg

import (
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-containerregistry/pkg/name"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

const (
	defaultVer = "latest"
)

var errInvalidPkgName = errors.New("invalid package dependency supplied")

// ValidDep --.
func ValidDep(pkg string) (bool, error) {
	upkg := strings.ReplaceAll(pkg, "@", ":")

	_, err := parsePackageReference(upkg)
	if err != nil {
		return false, errors.Errorf("%s: %s", errInvalidPkgName.Error(), err.Error())
	}

	return true, nil
}

func parsePackageReference(pkg string) (bool, error) { //nolint:gocyclo
	if pkg == "" {
		return false, errors.Errorf("could not parse reference: empty package name, %s", errInvalidPkgName.Error())
	}

	version := defaultVer
	var source string
	parts := strings.Split(pkg, "/")
	lastPart := parts[len(parts)-1]

	if strings.ContainsAny(lastPart, "@:") {
		var delimiter string
		if at := strings.Index(lastPart, "@"); at != -1 {
			delimiter = "@"
		}
		if colon := strings.LastIndex(lastPart, ":"); colon != -1 {
			if delimiter == "" || colon > strings.Index(lastPart, delimiter) {
				delimiter = ":"
			}
		}

		source = pkg
		if prefix, suffix, found := strings.Cut(lastPart, delimiter); found {
			parts[len(parts)-1] = prefix
			source = strings.Join(parts, "/")
			version = suffix
		}
	} else {
		source = pkg
	}

	_, err := name.ParseReference(source)
	if err != nil {
		return false, errors.Errorf("%s: %s", errInvalidPkgName.Error(), err.Error())
	}

	if version != defaultVer {
		_, err := semver.NewConstraint(version)
		if err != nil {
			return false, errors.Errorf("invalid SemVer constraint %s: %s", version, errInvalidPkgName.Error())
		}
	}

	return true, nil
}
