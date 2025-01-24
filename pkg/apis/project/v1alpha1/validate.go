// Copyright 2025 Upbound Inc.
// All rights reserved

// Package v1alpha1 contains v1alpha1 of the Upbound project type.
package v1alpha1

import (
	"path/filepath"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// Validate validates a project.
func (p *Project) Validate() error {
	var errs []error

	if p.GetName() == "" {
		errs = append(errs, errors.New("name must not be empty"))
	}
	if p.Spec == nil {
		errs = append(errs, errors.New("spec must be present"))
	} else {
		errs = append(errs, p.Spec.Validate())
	}

	return errors.Join(errs...)
}

// Validate validates a project's spec.
func (s *ProjectSpec) Validate() error {
	var errs []error

	if s.Repository == "" {
		errs = append(errs, errors.New("repository must not be empty"))
	}

	if s.Paths != nil {
		if s.Paths.APIs != "" && filepath.IsAbs(s.Paths.APIs) {
			errs = append(errs, errors.New("apis path must be relative"))
		}
		if s.Paths.Functions != "" && filepath.IsAbs(s.Paths.Functions) {
			errs = append(errs, errors.New("functions path must be relative"))
		}
		if s.Paths.Examples != "" && filepath.IsAbs(s.Paths.Examples) {
			errs = append(errs, errors.New("examples path must be relative"))
		}
		if s.Paths.Tests != "" && filepath.IsAbs(s.Paths.Tests) {
			errs = append(errs, errors.New("tests path must be relative"))
		}
	}

	if s.Architectures != nil && len(s.Architectures) == 0 {
		errs = append(errs, errors.New("architectures must not be empty"))
	}

	return errors.Join(errs...)
}

// Default applies defaults for a project.
func (p *Project) Default() {
	if p.Spec == nil {
		p.Spec = &ProjectSpec{}
	}

	p.Spec.Default()
}

// Default applies defaults for a project's spec.
func (s *ProjectSpec) Default() {
	if s.Paths == nil {
		s.Paths = &ProjectPaths{}
	}
	s.Paths.Default()

	if len(s.Architectures) == 0 {
		s.Architectures = []string{"amd64", "arm64"}
	}
}

// Default applies defaults to a project's paths.
func (p *ProjectPaths) Default() {
	if p.APIs == "" {
		p.APIs = "apis"
	}
	if p.Examples == "" {
		p.Examples = "examples"
	}
	if p.Functions == "" {
		p.Functions = "functions"
	}
	if p.Tests == "" {
		p.Tests = "tests"
	}
}
