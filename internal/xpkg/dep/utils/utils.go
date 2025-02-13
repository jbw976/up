// Copyright 2025 Upbound Inc.
// All rights reserved

package utils

import (
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"

	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
)

// IsDigest checks if the given constraint is a valid digest.
func IsDigest(d *v1beta1.Dependency) bool {
	_, err := name.NewDigest(fmt.Sprintf("%s@%s", d.Package, d.Constraints))
	return err == nil
}
