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
// +kubebuilder:storageversion
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
	ImageConfig            []ImageConfig                    `json:"imageConfig,omitempty"`
	// APIDependencies are the API dependencies for this project.
	// NOTE: This is an experimental feature and is subject to change.
	// +optional
	APIDependencies []APIDependencies `json:"apiDependencies,omitempty"`
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

// ImageMatch defines a rule for matching image.
type ImageMatch struct {
	// Type is the type of match.
	// +optional
	// +kubebuilder:validation:Enum=Prefix
	// +kubebuilder:default=Prefix
	Type string `json:"type"`

	// Prefix is the prefix that should be matched.
	Prefix string `json:"prefix"`
}

// ImageRewrite defines how a matched image should be rewritten.
type ImageRewrite struct {
	// Prefix is the prefix to use when rewriting the image.
	Prefix string `json:"prefix"`
}

// ImageConfig defines a set of rules for matching and rewriting images.
type ImageConfig struct {
	// MatchImages is a list of image matching rules that should be satisfied.
	// +kubebuilder:validation:XValidation:rule="size(self) > 0",message="matchImages should have at least one element."
	MatchImages []ImageMatch `json:"matchImages"`

	// RewriteImage defines how a matched image should be rewritten.
	RewriteImage ImageRewrite `json:"rewriteImage"`
}

// API dependency type constants.
const (
	// APIDependencyTypeK8s represents Kubernetes API dependencies.
	APIDependencyTypeK8s = "k8s"
	// APIDependencyTypeCRD represents Custom Resource Definition dependencies.
	APIDependencyTypeCRD = "crd"
)

// APIDependencies defines a reference to an external API dependency.
// NOTE: This is an experimental feature and is subject to change.
type APIDependencies struct {
	// Type defines the type of API dependency.
	// +kubebuilder:validation:Enum=k8s;crd
	Type string `json:"type"`

	// Git defines the git repository source for the API dependency.
	// +optional
	Git *APIGitReference `json:"git,omitempty"`

	// HTTP defines the HTTP source for the API dependency.
	// +optional
	HTTP *APIHTTPReference `json:"http,omitempty"`

	// K8s defines the Kubernetes API version for the dependency.
	// +optional
	K8s *APIK8sReference `json:"k8s,omitempty"`
}

// APIGitReference defines a git repository source for an API dependency.
type APIGitReference struct {
	// Repository is the git repository URL.
	Repository string `json:"repository"`

	// Ref is the git reference (branch, tag, or commit SHA).
	// +optional
	Ref string `json:"ref,omitempty"`

	// Path is the path within the repository to the API definition.
	// +optional
	Path string `json:"path,omitempty"`
}

// APIHTTPReference defines an HTTP source for an API dependency.
type APIHTTPReference struct {
	// URL is the HTTP/HTTPS URL to fetch the API dependency from.
	URL string `json:"url"`
}

// APIK8sReference defines a Kubernetes API version reference.
type APIK8sReference struct {
	// Version is the Kubernetes API version (e.g., "v1.33.0").
	Version string `json:"version"`
}
