// Copyright 2025 Upbound Inc.
// All rights reserved

package project

import (
	"os"

	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"

	"github.com/upbound/up/internal/yaml"
	"github.com/upbound/up/pkg/apis/project/v1alpha1"
)

// Update updates a project's on-disk metadata.
func Update(projFS afero.Fs, projFilePath string, fn func(*v1alpha1.Project)) error {
	proj, err := Parse(projFS, projFilePath)
	if err != nil {
		return err
	}

	fn(proj)
	bs, err := yaml.Marshal(proj)
	if err != nil {
		return errors.Wrap(err, "failed to marshal project metadata")
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
func UpsertDependency(proj *v1alpha1.Project, newDep pkgmetav1.Dependency) error {
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
