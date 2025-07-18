// Copyright 2025 Upbound Inc.
// All rights reserved

package project

import (
	"os"

	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"

	"github.com/upbound/up/internal/yaml"
	"github.com/upbound/up/pkg/apis/project"
	"github.com/upbound/up/pkg/apis/project/v2alpha1"
)

// Update updates a project's on-disk metadata while preserving its original version.
func Update(projFS afero.Fs, projFilePath string, fn func(*v2alpha1.Project)) error {
	// Parse the project file to detect its version
	versionedProject, err := project.ParseVersioned(projFS, projFilePath)
	if err != nil {
		return err
	}

	// Work with v2alpha1 representation
	var v2Project *v2alpha1.Project
	isV1 := false

	switch versionedProject.Version {
	case project.VersionV1Alpha1:
		isV1 = true
		// Convert v1 to v2 for the update function
		v2Project, err = project.ConvertToV2WithoutDefaults(versionedProject.V1)
		if err != nil {
			return err
		}
	case project.VersionV2Alpha1:
		v2Project = versionedProject.V2
	default:
		return errors.Errorf("unsupported project version: %s", versionedProject.Version)
	}

	// Apply the update function
	fn(v2Project)

	// Marshal back to the original version
	var bs []byte
	if isV1 {
		// Convert back to v1alpha1 to preserve the original version
		v1Project, err := project.ConvertToV1(v2Project)
		if err != nil {
			return err
		}
		bs, err = yaml.Marshal(v1Project)
		if err != nil {
			return errors.Wrap(err, "failed to marshal v1alpha1 project metadata")
		}
	} else {
		// Keep as v2alpha1
		bs, err = yaml.Marshal(v2Project)
		if err != nil {
			return errors.Wrap(err, "failed to marshal v2alpha1 project metadata")
		}
	}

	// Keep the permissions on the meta file the same if it already exists.
	perms := os.FileMode(0o644)
	st, err := projFS.Stat(projFilePath)
	if err == nil {
		perms = st.Mode()
	}

	return errors.Wrap(afero.WriteFile(projFS, projFilePath, bs, perms), "failed to write project metadata")
}

// UpsertDependency adds or updates a dependency in the project's metadata.
func UpsertDependency(proj *v2alpha1.Project, newDep pkgmetav1.Dependency) error {
	newDep, err := NormalizeDependency(newDep)
	if err != nil {
		return err
	}

	updated := false
	for i, dep := range proj.Spec.DependsOn {
		dep, err = NormalizeDependency(dep)
		if err != nil {
			return err
		}

		if *dep.Package != *newDep.Package {
			continue
		}
		if updated {
			return errors.New("project contains duplicate dependencies")
		}
		dep.Version = newDep.Version
		proj.Spec.DependsOn[i] = dep
		updated = true
	}

	if !updated {
		// Dependency is new to the project.
		proj.Spec.DependsOn = append(proj.Spec.DependsOn, newDep)
	}

	return nil
}
