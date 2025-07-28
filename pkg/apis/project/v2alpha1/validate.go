// Copyright 2025 Upbound Inc.
// All rights reserved

// Package v2alpha1 contains v2alpha1 of the Upbound project type.
package v2alpha1

import (
	"path/filepath"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
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
		if s.Paths.Operations != "" && filepath.IsAbs(s.Paths.Operations) {
			errs = append(errs, errors.New("operations path must be relative"))
		}
	}

	if s.Architectures != nil && len(s.Architectures) == 0 {
		errs = append(errs, errors.New("architectures must not be empty"))
	}

	// Validate API dependencies
	for i, dep := range s.APIDependencies {
		if err := dep.Validate(); err != nil {
			errs = append(errs, errors.Wrapf(err, "api dependency %d", i))
		}
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

	if s.Crossplane == nil {
		s.Crossplane = &pkgmetav1.CrossplaneConstraints{
			Version: ">=v2.0.0-rc.0",
		}
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
	if p.Operations == "" {
		p.Operations = "operations"
	}
}

// Validate validates an API dependency.
func (d *APIDependencies) Validate() error {
	var errs []error

	if d.Type == "" {
		errs = append(errs, errors.New("type must not be empty"))
	}

	// Count non-nil sources
	sourceCount := 0
	if d.Git != nil {
		sourceCount++
		if err := d.Git.Validate(); err != nil {
			errs = append(errs, errors.Wrap(err, "git"))
		}
	}
	if d.HTTP != nil {
		sourceCount++
		if err := d.HTTP.Validate(); err != nil {
			errs = append(errs, errors.Wrap(err, "http"))
		}
	}
	if d.K8s != nil {
		sourceCount++
		if err := d.K8s.Validate(); err != nil {
			errs = append(errs, errors.Wrap(err, "k8s"))
		}
	}

	// Ensure exactly one source is specified
	if sourceCount == 0 {
		errs = append(errs, errors.New("exactly one source (git, http, or k8s) must be specified"))
	} else if sourceCount > 1 {
		errs = append(errs, errors.New("only one source (git, http, or k8s) may be specified"))
	}

	return errors.Join(errs...)
}

// Validate validates a git API reference.
func (g *APIGitReference) Validate() error {
	var errs []error

	if g.Repository == "" {
		errs = append(errs, errors.New("repository must not be empty"))
	}

	return errors.Join(errs...)
}

// Validate validates an HTTP API reference.
func (h *APIHTTPReference) Validate() error {
	var errs []error

	if h.URL == "" {
		errs = append(errs, errors.New("url must not be empty"))
	}

	return errors.Join(errs...)
}

// Validate validates a Kubernetes API reference.
func (k *APIK8sReference) Validate() error {
	var errs []error

	if k.Version == "" {
		errs = append(errs, errors.New("version must not be empty"))
	}

	return errors.Join(errs...)
}
