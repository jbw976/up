// Copyright 2025 Upbound Inc.
// All rights reserved

// Package project contains functions for project.
package project

import (
	"github.com/upbound/up/pkg/apis/project/v1alpha1"
	"github.com/upbound/up/pkg/apis/project/v2alpha1"
)

// Version represents the different project API versions.
type Version string

const (
	// VersionV1Alpha1 represents the v1alpha1 project version.
	VersionV1Alpha1 Version = v1alpha1.GroupVersion
	// VersionV2Alpha1 represents the v2alpha1 project version.
	VersionV2Alpha1 Version = v2alpha1.GroupVersion
)

// Interface represents common methods across project versions.
type Interface interface {
	Validate() error
	Default()
}

// Versioned wraps a project with version information.
type Versioned struct {
	Version Version
	V1      *v1alpha1.Project
	V2      *v2alpha1.Project
}

// GetProject returns the underlying project interface.
func (p *Versioned) GetProject() Interface {
	switch p.Version {
	case VersionV1Alpha1:
		return p.V1
	case VersionV2Alpha1:
		return p.V2
	default:
		return nil
	}
}

// IsV1 returns true if the project is v1alpha1.
func (p *Versioned) IsV1() bool {
	return p.Version == VersionV1Alpha1
}

// IsV2 returns true if the project is v2alpha1.
func (p *Versioned) IsV2() bool {
	return p.Version == VersionV2Alpha1
}
