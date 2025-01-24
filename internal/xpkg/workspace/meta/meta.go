// Copyright 2025 Upbound Inc.
// All rights reserved

// Package meta contains abstractions for working with package metadata.
package meta

import (
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	pkgmetav1alpha1 "github.com/crossplane/crossplane/apis/pkg/meta/v1alpha1"
	pkgmetav1beta1 "github.com/crossplane/crossplane/apis/pkg/meta/v1beta1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"

	"github.com/upbound/up/internal/xpkg/dep/manager"
	"github.com/upbound/up/internal/xpkg/scheme"
	"github.com/upbound/up/internal/yaml"
	projectv1alpha1 "github.com/upbound/up/pkg/apis/project/v1alpha1"
)

const (
	errInvalidMetaFile           = "invalid meta type supplied"
	errMetaContainsDupeDep       = "meta file contains duplicate dependency"
	errUnsupportedPackageVersion = "unsupported package version supplied"
	errInvalidDep                = "meta file contains invalid dependency"
)

// Meta provides helpful methods for interacting with a metafile's
// runtime.Object.
type Meta struct {
	// obj is the runtime.Object representation of the meta file.
	obj runtime.Object
}

// New constructs a new Meta given a.
func New(obj runtime.Object) *Meta {
	return &Meta{
		obj: obj,
	}
}

// DependsOn returns a slice of v1beta1.Dependency that this workspace depends on.
func (m *Meta) DependsOn() ([]v1beta1.Dependency, error) {
	pkg, ok := scheme.TryConvertToPkg(m.obj,
		&pkgmetav1.Provider{},
		&pkgmetav1.Configuration{},
		&pkgmetav1.Function{},
		&projectv1alpha1.Project{},
	)
	if !ok {
		return nil, errors.New(errUnsupportedPackageVersion)
	}

	out := make([]v1beta1.Dependency, len(pkg.GetDependencies()))
	for i, d := range pkg.GetDependencies() {
		dep, ok := manager.ConvertToV1beta1(d)
		if !ok {
			return nil, errors.New(errInvalidDep)
		}
		out[i] = dep
	}

	return out, nil
}

// Upsert will add an entry to the meta file, if the meta file exists and
// does not yet have an entry for the given package. If an entry does exist,
// the entry will be updated to the given package version.
func (m *Meta) Upsert(d v1beta1.Dependency) error {
	return upsertDeps(d, m.obj)
}

// Bytes returns the cleaned up byte representation of the meta file obj.
func (m *Meta) Bytes() ([]byte, error) {
	return yaml.Marshal(m.obj)
}

// Object returns the raw meta object.
func (m *Meta) Object() runtime.Object {
	return m.obj
}

// upsertDeps takes a v1beta1.Dependency and a runtime.Object of type that can
// be converted to a v1.Pkg and returns an updated runtime.Object with a slice
// of dependencies that includes the provided dependency d.
func upsertDeps(d v1beta1.Dependency, o runtime.Object) error {
	p, ok := scheme.TryConvertToPkg(o,
		&pkgmetav1.Provider{},
		&pkgmetav1.Configuration{},
		&pkgmetav1.Function{},
		&projectv1alpha1.Project{},
	)
	if !ok {
		return errors.New(errUnsupportedPackageVersion)
	}
	deps := p.GetDependencies()

	processed := false
	for i := range deps {
		// modify the underlying slice
		dep := deps[i]
		switch {
		case dep.Provider != nil && *dep.Provider == d.Package:
			if processed {
				return errors.New(errMetaContainsDupeDep)
			}
			deps[i].Version = d.Constraints
			processed = true
		case dep.Configuration != nil && *dep.Configuration == d.Package:
			if processed {
				return errors.New(errMetaContainsDupeDep)
			}
			deps[i].Version = d.Constraints
			processed = true
		case dep.Function != nil && *dep.Function == d.Package:
			if processed {
				return errors.New(errMetaContainsDupeDep)
			}
			deps[i].Version = d.Constraints
			processed = true
		}
	}

	if !processed {
		dep := pkgmetav1.Dependency{
			Version: d.Constraints,
		}

		switch d.Type {
		case v1beta1.ProviderPackageType:
			dep.Provider = &d.Package
		case v1beta1.ConfigurationPackageType:
			dep.Configuration = &d.Package
		case v1beta1.FunctionPackageType:
			dep.Function = &d.Package
		}

		deps = append(deps, dep)
	}

	switch v := o.(type) {
	case *pkgmetav1alpha1.Configuration:
		v.Spec.DependsOn = convertToV1alpha1(deps)
	case *pkgmetav1.Configuration:
		v.Spec.DependsOn = deps
	case *pkgmetav1alpha1.Provider:
		v.Spec.DependsOn = convertToV1alpha1(deps)
	case *pkgmetav1.Provider:
		v.Spec.DependsOn = deps
	case *pkgmetav1beta1.Function:
		v.Spec.DependsOn = convertToV1beta1(deps)
	case *pkgmetav1.Function:
		v.Spec.DependsOn = deps
	case *projectv1alpha1.Project:
		v.Spec.DependsOn = deps
	}

	return nil
}

func convertToV1alpha1(deps []pkgmetav1.Dependency) []pkgmetav1alpha1.Dependency {
	alphaDeps := make([]pkgmetav1alpha1.Dependency, 0)
	for _, d := range deps {
		alphaDeps = append(alphaDeps, manager.MetaConvertToV1alpha1(d))
	}
	return alphaDeps
}

func convertToV1beta1(deps []pkgmetav1.Dependency) []pkgmetav1beta1.Dependency {
	betaDeps := make([]pkgmetav1beta1.Dependency, 0)
	for _, d := range deps {
		betaDeps = append(betaDeps, manager.MetaConvertToV1beta1(d))
	}
	return betaDeps
}
