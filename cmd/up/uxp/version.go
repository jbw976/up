// Copyright 2025 Upbound Inc.
// All rights reserved

package uxp

import (
	"fmt"
	"strings"

	"github.com/blang/semver/v4"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
)

type version string

//nolint:gochecknoglobals // Would make this a const if we could.
var uxpV2Version = semver.MustParse("2.0.0-up.0")

// Validate implements the kong Validatable interface, see https://github.com/alecthomas/kong?tab=readme-ov-file#validation.
// Unfortunately it's not an exposed interface, so we can't enforce version to implement it at build time.
func (v *version) Validate() error {
	// version should not start with a v
	vs := string(*v)
	if strings.HasPrefix(vs, "v") {
		return fmt.Errorf("versions should not start with a v: %s", vs)
	}
	pv, err := semver.Parse(vs)
	if err != nil {
		return errors.Wrapf(err, "unable to parse version %s", vs)
	}
	if pv.LT(uxpV2Version) {
		return fmt.Errorf("this command only supports UXP versions >= %v, please use the helm chart directly if you want to install UXP v1", uxpV2Version)
	}
	return nil
}
