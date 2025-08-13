// Copyright 2025 Upbound Inc.
// All rights reserved

package v2alpha1

import pkgmetav1 "github.com/crossplane/crossplane/v2/apis/pkg/meta/v1"

var _ pkgmetav1.Pkg = &Project{}

// GetCrossplaneConstraints gets the Project's Crossplane version constraints.
func (p *Project) GetCrossplaneConstraints() *pkgmetav1.CrossplaneConstraints {
	return p.Spec.Crossplane
}

// GetDependencies gets the Project's dependencies.
func (p *Project) GetDependencies() []pkgmetav1.Dependency {
	return p.Spec.DependsOn
}

// GetCapabilities returns function capabilities.
func (p *Project) GetCapabilities() []string {
	return nil
}
