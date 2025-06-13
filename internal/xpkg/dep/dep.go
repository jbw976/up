// Copyright 2025 Upbound Inc.
// All rights reserved

// Package dep contains functions to work with dependencies.
package dep

import (
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"k8s.io/utils/ptr"

	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"

	"github.com/upbound/up/internal/xpkg/dep/resolver/image"
)

// New returns a new v1beta1.Dependency based on the given package name.
// Expects names of the form source@version or source:version where
// the version part can be left blank to indicate 'latest'.
func New(pkg string) v1beta1.Dependency {
	// If the passed-in version was blank, use the default to pass
	// constraint checks and grab the latest semver
	version := image.DefaultVer

	// Split the package into parts by '/'
	parts := strings.Split(pkg, "/")

	// Assume the last part could be the version tag
	lastPart := parts[len(parts)-1]

	// Initialize source with the input package name
	source := pkg

	// Check if the last part contains '@' or ':'
	if strings.ContainsAny(lastPart, "@:") {
		// Find the first occurrence of either '@' or ':'
		var delimiter string
		if at := strings.Index(lastPart, "@"); at != -1 {
			delimiter = "@"
		}
		if colon := strings.LastIndex(lastPart, ":"); colon != -1 {
			// Use the latest delimiter found
			if delimiter == "" || colon > strings.Index(lastPart, delimiter) {
				delimiter = ":"
			}
		}

		if prefix, suffix, found := strings.Cut(lastPart, delimiter); found {
			parts[len(parts)-1] = prefix
			source = strings.Join(parts, "/")
			version = suffix
		}
	}

	return v1beta1.Dependency{
		Package:     source,
		Constraints: version,
	}
}

// ToMetaDependency returns a metadata dependency for the given dependency.
func ToMetaDependency(d v1beta1.Dependency) pkgmetav1.Dependency {
	if d.Type != nil {
		switch *d.Type {
		case v1beta1.ConfigurationPackageType:
			d.APIVersion = ptr.To(pkgv1.ConfigurationGroupVersionKind.GroupVersion().String())
			d.Kind = &pkgv1.ConfigurationKind

		case v1beta1.ProviderPackageType:
			d.APIVersion = ptr.To(pkgv1.ProviderGroupVersionKind.GroupVersion().String())
			d.Kind = &pkgv1.ProviderKind

		case v1beta1.FunctionPackageType:
			d.APIVersion = ptr.To(pkgv1.FunctionGroupVersionKind.GroupVersion().String())
			d.Kind = &pkgv1.FunctionKind
		}

		d.Type = nil
	}

	return pkgmetav1.Dependency{
		APIVersion: d.APIVersion,
		Kind:       d.Kind,
		Package:    &d.Package,
		Version:    d.Constraints,
	}
}

// NewWithType returns a new v1beta1.Dependency based on the given package
// name and PackageType (represented as a string).
// Expects names of the form source@version where @version can be
// left blank in order to indicate 'latest'.
func NewWithType(pkg string, t string) v1beta1.Dependency {
	d := New(pkg)

	c := cases.Title(language.Und) // Create a caser for title casing
	normalized := c.String(strings.ToLower(t))

	switch normalized {
	case string(v1beta1.ConfigurationPackageType):
		d.Type = ptr.To(v1beta1.ConfigurationPackageType)
	case string(v1beta1.FunctionPackageType):
		d.Type = ptr.To(v1beta1.FunctionPackageType)
	case string(v1beta1.ProviderPackageType):
		d.Type = ptr.To(v1beta1.ProviderPackageType)
	}

	return d
}
