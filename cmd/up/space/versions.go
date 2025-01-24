// Copyright 2025 Upbound Inc.
// All rights reserved

package space

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
)

const (
	// chartAnnotationUpVersion is the annotation on a chart that is used to
	// constraint which version of Upbound up can be used to install it.
	// It's a semicolon-separated list of semver constraints with a message:
	//
	// spaces.upbound.io/up-version-constraints: ">= 0.20: up 0.20.0 or later is required; <0.29: up <0.29 is required".
	chartAnnotationUpConstraints = "spaces.upbound.io/up-version-constraints"
)

type constraint struct {
	// semver is the semver constraint to check against.
	semver string

	// message is the message to display if the constraint is not met. This should
	// repeat the semver, but in a more human-readable format, and
	// actionable by the user.
	message string
}

var (
	// initVersionConstraints is the list of version constraints that are checked
	// on up init.
	initVersionConstraints = []constraint{
		{semver: ">= 1.0-0", message: "target version must be 1.0 or later. Use up < 0.20.0 to install earlier versions."},
	}

	// upgradeVersionConstraints is the list of version constraints that are
	// checked on up upgrade.
	upgradeVersionConstraints = []constraint{
		{semver: ">= 1.0-0", message: "target version must be 1.0 or later. Use up < 0.20.0 to install earlier versions."},
	}

	// upgradeFromVersionConstraints is the list of version constraints that are
	// checked on up upgrade against the installed version on the customer
	// host cluster.
	upgradeFromVersionConstraints = []constraint{}
)

func parseChartUpConstraints(s string) ([]constraint, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}

	clauses := strings.Split(s, ";")
	constraints := make([]constraint, 0, len(clauses))
	for _, c := range clauses {
		parts := strings.SplitN(c, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid constraint %q in the chart", c)
		}
		constraints = append(constraints, constraint{semver: strings.TrimSpace(parts[0]), message: strings.TrimSpace(parts[1])})
	}
	return constraints, nil
}

func checkVersion(msg string, constraints []constraint, v string) error {
	sv, err := semver.NewVersion(v)
	if err != nil {
		return fmt.Errorf("failed to parse version %q: %w", v, err)
	}

	for _, vc := range constraints {
		c, err := semver.NewConstraint(vc.semver)
		if err != nil {
			return fmt.Errorf("failed to parse constraint %q: %w", vc.semver, err)
		}

		if !c.Check(sv) {
			return fmt.Errorf("%s: %s", msg, vc.message)
		}
	}

	return nil
}
