// Copyright 2025 Upbound Inc.
// All rights reserved

package project

import (
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/yaml"
	"github.com/upbound/up/pkg/apis/project"
	"github.com/upbound/up/pkg/apis/project/v2alpha1"
)

// UpgradeToV2 upgrades a project file from v1alpha1 to v2alpha1.
func UpgradeToV2(projFS afero.Fs, projFilePath string) error {
	vproj, err := project.ParseVersioned(projFS, projFilePath)
	if err != nil {
		return errors.Wrap(err, "failed to parse project")
	}

	// Skip if already v2alpha1
	if vproj.Version == project.VersionV2Alpha1 {
		return errors.New("project is already v2alpha1")
	}

	// Convert to v2alpha1
	v2Project, err := project.ConvertToV2WithoutDefaults(vproj.V1)
	if err != nil {
		return errors.Wrap(err, "failed to convert v1alpha1 to v2alpha1")
	}

	// Set proper API version and kind
	v2Project.APIVersion = v2alpha1.GroupVersion
	v2Project.Kind = v2alpha1.ProjectKind

	// Marshal to YAML
	updatedYAML, err := yaml.Marshal(v2Project)
	if err != nil {
		return errors.Wrap(err, "failed to marshal upgraded project to YAML")
	}

	// Write back to file
	if err := afero.WriteFile(projFS, projFilePath, updatedYAML, 0o644); err != nil {
		return errors.Wrap(err, "failed to write upgraded project file")
	}

	return nil
}
