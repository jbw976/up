// Copyright 2025 Upbound Inc.
// All rights reserved

package v2alpha1

import (
	"sigs.k8s.io/controller-runtime/pkg/conversion"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/pkg/apis/project/v1alpha1"
)

// Hub marks this type as the conversion hub.
func (p *Project) Hub() {}

// ConvertTo converts this v2alpha1 Project to the Hub version (v1alpha1).
func (p *Project) ConvertTo(dstRaw conversion.Hub) error {
	dst, ok := dstRaw.(*v1alpha1.Project)
	if !ok {
		return errors.Errorf("expected *v1alpha1.Project, got %T", dstRaw)
	}

	// Marshal v2alpha1 to YAML
	data, err := yaml.Marshal(p)
	if err != nil {
		return errors.Wrap(err, "failed to marshal v2alpha1 Project")
	}

	// Unmarshal into v1alpha1
	if err := yaml.Unmarshal(data, dst); err != nil {
		return errors.Wrap(err, "failed to unmarshal into v1alpha1 Project")
	}

	// Handle version-specific differences
	dst.TypeMeta.APIVersion = v1alpha1.GroupVersion

	return nil
}

// ConvertFrom converts from the Hub version (v1alpha1) to this v2alpha1 Project.
func (p *Project) ConvertFrom(srcRaw conversion.Hub) error {
	src, ok := srcRaw.(*v1alpha1.Project)
	if !ok {
		return errors.Errorf("expected *v1alpha1.Project, got %T", srcRaw)
	}

	// Marshal v1alpha1 to YAML
	data, err := yaml.Marshal(src)
	if err != nil {
		return errors.Wrap(err, "failed to marshal v1alpha1 Project")
	}

	// Unmarshal into v2alpha1
	if err := yaml.Unmarshal(data, p); err != nil {
		return errors.Wrap(err, "failed to unmarshal into v2alpha1 Project")
	}

	// Handle version-specific differences
	p.TypeMeta.APIVersion = GroupVersion

	return nil
}
