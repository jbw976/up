// Copyright 2025 Upbound Inc.
// All rights reserved

package v1alpha1

import (
	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// Validate checks if the CompositionTest resource is properly configured.
func (c *CompositionTest) Validate() error {
	var errs []error

	// Validate Spec
	errs = append(errs, c.Spec.validateCompositionTestSpec()...)

	// Return combined errors
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// validateCompositionTestSpec ensures the CompositionTestSpec is valid.
func (s *CompositionTestSpec) validateCompositionTestSpec() []error {
	var errs []error

	// Ensure mutually exclusive fields are not set together
	if s.XR.Raw != nil && s.XRPath != "" {
		errs = append(errs, errors.New("only one of 'xr' or 'xrPath' may be specified"))
	}
	if s.XRD.Raw != nil && s.XRDPath != "" {
		errs = append(errs, errors.New("only one of 'xrd' or 'xrdPath' may be specified"))
	}
	if s.Composition.Raw != nil && s.CompositionPath != "" {
		errs = append(errs, errors.New("only one of 'composition' or 'compositionPath' may be specified"))
	}

	return errs
}
