// Copyright 2025 Upbound Inc.
// All rights reserved

package project

import (
	"github.com/spf13/afero"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/yaml"
	"github.com/upbound/up/pkg/apis/project/v1alpha1"
	"github.com/upbound/up/pkg/apis/project/v2alpha1"
)

// DetectVersion detects the project version from the YAML content.
func DetectVersion(projFS afero.Fs, projFilePath string) (Version, error) {
	projYAML, err := afero.ReadFile(projFS, projFilePath)
	if err != nil {
		return "", errors.Wrapf(err, "failed to read project file %q", projFilePath)
	}

	var typeMeta metav1.TypeMeta
	if err := yaml.Unmarshal(projYAML, &typeMeta); err != nil {
		return "", errors.Wrap(err, "failed to parse project file TypeMeta")
	}

	switch typeMeta.APIVersion {
	case v1alpha1.GroupVersion, v1alpha1.Version:
		return VersionV1Alpha1, nil
	case v2alpha1.GroupVersion, v2alpha1.Version:
		return VersionV2Alpha1, nil
	case "":
		// Default to v1alpha1 for backward compatibility
		return VersionV1Alpha1, nil
	default:
		return "", errors.Errorf("unsupported project API version: %s", typeMeta.APIVersion)
	}
}

// ParseVersioned parses a project file and returns a version-aware project.
func ParseVersioned(projFS afero.Fs, projFilePath string) (*Versioned, error) {
	version, err := DetectVersion(projFS, projFilePath)
	if err != nil {
		return nil, err
	}

	projYAML, err := afero.ReadFile(projFS, projFilePath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read project file %q", projFilePath)
	}

	result := &Versioned{Version: version}

	switch version {
	case VersionV1Alpha1:
		var project v1alpha1.Project
		if err := yaml.Unmarshal(projYAML, &project); err != nil {
			return nil, errors.Wrap(err, "failed to parse v1alpha1 project file")
		}
		if err := project.Validate(); err != nil {
			return nil, errors.Wrap(err, "invalid v1alpha1 project file")
		}
		result.V1 = &project

	case VersionV2Alpha1:
		var project v2alpha1.Project
		if err := yaml.Unmarshal(projYAML, &project); err != nil {
			return nil, errors.Wrap(err, "failed to parse v2alpha1 project file")
		}
		if err := project.Validate(); err != nil {
			return nil, errors.Wrap(err, "invalid v2alpha1 project file")
		}
		result.V2 = &project

	default:
		return nil, errors.Errorf("unsupported project version: %s", version)
	}

	return result, nil
}

// ConvertToV2 converts a v1alpha1 project to v2alpha1.
func ConvertToV2(v1Project *v1alpha1.Project) (*v2alpha1.Project, error) {
	v2Project := &v2alpha1.Project{}
	if err := v2Project.ConvertFrom(v1Project); err != nil {
		return nil, errors.Wrap(err, "failed to convert v1alpha1 project to v2alpha1")
	}
	return v2Project, nil
}

// ConvertToV1 converts a v2alpha1 project to v1alpha1 with defaults.
func ConvertToV1(v2Project *v2alpha1.Project) (*v1alpha1.Project, error) {
	// Apply defaults to v2 project before conversion
	v2Project.Default()

	v1Project := &v1alpha1.Project{}
	if err := v2Project.ConvertTo(v1Project); err != nil {
		return nil, errors.Wrap(err, "failed to convert v2alpha1 project to v1alpha1")
	}
	return v1Project, nil
}

// ConvertToV1WithoutDefaults converts a v2alpha1 project to v1alpha1.
func ConvertToV1WithoutDefaults(v2Project *v2alpha1.Project) (*v1alpha1.Project, error) {
	v1Project := &v1alpha1.Project{}
	if err := v2Project.ConvertTo(v1Project); err != nil {
		return nil, errors.Wrap(err, "failed to convert v2alpha1 project to v1alpha1")
	}
	return v1Project, nil
}
