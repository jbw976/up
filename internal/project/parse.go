// Copyright 2025 Upbound Inc.
// All rights reserved

package project

import (
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/pkg/apis/project"
	"github.com/upbound/up/pkg/apis/project/v1alpha1"
)

// WithVersion wraps a project with version information.
type WithVersion struct {
	*v1alpha1.Project
	version project.Version
}

// IsV1 returns true if the project is v1alpha1.
func (p *WithVersion) IsV1() bool {
	return p.version == project.VersionV1Alpha1
}

// IsV2 returns true if the project is v2alpha1.
func (p *WithVersion) IsV2() bool {
	return p.version == project.VersionV2Alpha1
}

// Parse parses and validates the project file, returning a v1alpha1 project.
func Parse(projFS afero.Fs, projFilePath string) (*v1alpha1.Project, error) {
	projectWithVersion, err := ParseWithVersion(projFS, projFilePath)
	if err != nil {
		return nil, err
	}
	return projectWithVersion.Project, nil
}

// ParseWithVersion parses and validates the project file, returning a v1alpha1 project with version info.
func ParseWithVersion(projFS afero.Fs, projFilePath string) (*WithVersion, error) {
	versionedProject, err := project.ParseVersioned(projFS, projFilePath)
	if err != nil {
		return nil, err
	}

	result := &WithVersion{version: versionedProject.Version}

	switch versionedProject.Version {
	case project.VersionV1Alpha1:
		result.Project = versionedProject.V1
	case project.VersionV2Alpha1:
		v1, err := project.ConvertToV1(versionedProject.V2)
		if err != nil {
			return nil, err
		}
		result.Project = v1
	default:
		return nil, errors.Errorf("unsupported project version: %s", versionedProject.Version)
	}

	return result, nil
}
