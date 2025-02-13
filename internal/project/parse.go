// Copyright 2025 Upbound Inc.
// All rights reserved

package project

import (
	"github.com/spf13/afero"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/pkg/apis/project/v1alpha1"
)

// Parse parses and validates the project file.
func Parse(projFS afero.Fs, projFilePath string) (*v1alpha1.Project, error) {
	// Parse and validate the project file.
	projYAML, err := afero.ReadFile(projFS, projFilePath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read project file %q", projFilePath)
	}
	var project v1alpha1.Project
	err = yaml.Unmarshal(projYAML, &project)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse project file")
	}
	if err := project.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid project file")
	}
	return &project, nil
}
