// Copyright 2025 Upbound Inc.
// All rights reserved

package project

import (
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	v1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"

	"github.com/upbound/up/internal/yaml"
	"github.com/upbound/up/pkg/apis/project/v1alpha1"
	"github.com/upbound/up/pkg/apis/project/v2alpha1"
)

// ParseVersioned parses a project file and returns a version-aware project.
func ParseVersioned(projFS afero.Fs, projFilePath string) (*Versioned, error) {
	projYAML, err := afero.ReadFile(projFS, projFilePath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read project file %q", projFilePath)
	}

	var u unstructured.Unstructured
	if err := yaml.Unmarshal(projYAML, &u); err != nil {
		return nil, errors.Wrap(err, "failed to parse project file")
	}

	switch u.GetAPIVersion() {
	case string(VersionV1Alpha1):
		var project v1alpha1.Project
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &project); err != nil {
			return nil, errors.Wrap(err, "failed to convert v1alpha1 project from unstructured")
		}
		if err := project.Validate(); err != nil {
			return nil, errors.Wrap(err, "invalid v1alpha1 project file")
		}

		return &Versioned{Version: VersionV1Alpha1, V1: &project}, nil

	case string(VersionV2Alpha1):
		var project v2alpha1.Project
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &project); err != nil {
			return nil, errors.Wrap(err, "failed to convert v2alpha1 project from unstructured")
		}
		if err := project.Validate(); err != nil {
			return nil, errors.Wrap(err, "invalid v2alpha1 project file")
		}

		return &Versioned{Version: VersionV2Alpha1, V2: &project}, nil

	default:
		return nil, errors.Errorf("unsupported project API version: %s", u.GetAPIVersion())
	}
}

// ConvertToV1 converts a v2alpha1 project to v1alpha1.
func ConvertToV1(v2Project *v2alpha1.Project) (*v1alpha1.Project, error) {
	v1Project := &v1alpha1.Project{}
	if err := v1Project.ConvertFrom(v2Project); err != nil {
		return nil, errors.Wrap(err, "failed to convert v2alpha1 project to v1alpha1")
	}
	return v1Project, nil
}

// ConvertToV2 converts a v1alpha1 project to v2alpha1 with defaults.
func ConvertToV2(v1Project *v1alpha1.Project) (*v2alpha1.Project, error) {
	// Apply the default crossplane constraint default to the v1 project if
	// necessary, since v2 has different defaults.
	if v1Project.Spec.Crossplane == nil {
		v1Project.Spec.Crossplane = &v1.CrossplaneConstraints{
			Version: ">=v1.18.0 || >=v2.0.0-rc.0",
		}
	}

	v2Project := &v2alpha1.Project{}
	if err := v1Project.ConvertTo(v2Project); err != nil {
		return nil, errors.Wrap(err, "failed to convert v1alpha1 project to v2alpha1")
	}

	// Apply defaults to v2 project after conversion, since it has new fields.
	v2Project.Default()

	return v2Project, nil
}

// ConvertToV2WithoutDefaults converts a v1alpha1 project to v2alpha1.
func ConvertToV2WithoutDefaults(v1Project *v1alpha1.Project) (*v2alpha1.Project, error) {
	v2Project := &v2alpha1.Project{}
	if err := v1Project.ConvertTo(v2Project); err != nil {
		return nil, errors.Wrap(err, "failed to convert v1alpha1 project to v2alpha1")
	}
	return v2Project, nil
}
