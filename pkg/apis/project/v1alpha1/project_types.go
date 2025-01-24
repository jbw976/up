// Copyright 2025 Upbound Inc.
// All rights reserved

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
)

// Project defines an Upbound Project, which can be built into a Crossplane
// Configuration.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Project struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec *ProjectSpec `json:"spec,omitempty"`
}

// ProjectSpec is the spec for a Project. Since a Project is not a Kubernetes
// resource there is no Status, only Spec.
//
// +k8s:deepcopy-gen=true
type ProjectSpec struct {
	ProjectPackageMetadata `json:",inline"`
	Repository             string                           `json:"repository"`
	Crossplane             *pkgmetav1.CrossplaneConstraints `json:"crossplane,omitempty"`
	DependsOn              []pkgmetav1.Dependency           `json:"dependsOn,omitempty"`
	Paths                  *ProjectPaths                    `json:"paths,omitempty"`
	Architectures          []string                         `json:"architectures,omitempty"`
}

// ProjectPackageMetadata holds metadata about the project, which will become
// package metadata when a project is built into a Crossplane package.
type ProjectPackageMetadata struct {
	Maintainer  string `json:"maintainer,omitempty"`
	Source      string `json:"source,omitempty"`
	License     string `json:"license,omitempty"`
	Description string `json:"description,omitempty"`
	Readme      string `json:"readme,omitempty"`
}

// ProjectPaths configures the locations of various parts of the project, for
// use at build time.
type ProjectPaths struct {
	// APIs is the directory holding the project's apis. If not
	// specified, it defaults to `apis/`.
	APIs string `json:"apis,omitempty"`
	// Functions is the directory holding the project's functions. If not
	// specified, it defaults to `functions/`.
	Functions string `json:"functions,omitempty"`
	// Examples is the directory holding the project's examples. If not
	// specified, it defaults to `examples/`.
	Examples string `json:"examples,omitempty"`
	// Tests is the directory holding the project's tests. If not
	// specified, it defaults to `tests/`.
	Tests string `json:"tests,omitempty"`
}
