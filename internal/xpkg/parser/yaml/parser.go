// Copyright 2025 Upbound Inc.
// All rights reserved

// Package yaml contains a yaml package parser.
package yaml

import (
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/parser"

	"github.com/upbound/up/internal/xpkg/scheme"
)

const (
	errBuildMetaScheme   = "failed to build meta scheme for package parser"
	errBuildObjectScheme = "failed to build object scheme for package parser"
)

// New returns a new PackageParser that targets yaml files.
func New() (*parser.PackageParser, error) {
	metaScheme, err := scheme.BuildMetaScheme()
	if err != nil {
		return nil, errors.New(errBuildMetaScheme)
	}
	objScheme, err := scheme.BuildObjectScheme()
	if err != nil {
		return nil, errors.New(errBuildObjectScheme)
	}

	return parser.New(metaScheme, objScheme), nil
}
