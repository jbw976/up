// Copyright 2025 Upbound Inc.
// All rights reserved

package v1alpha1

import (
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
)

// Validate checks if the OperationTest resource is properly configured.
func (o *OperationTest) Validate() error {
	var errs []error

	// Validate Spec
	errs = append(errs, o.Spec.validateOperationTestSpec()...)

	// Return combined errors
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// validateOperationTestSpec ensures the OperationTestSpec is valid.
func (s *OperationTestSpec) validateOperationTestSpec() []error {
	var errs []error

	// Ensure mutually exclusive fields are not set together
	if len(s.RequiredResources) > 0 && s.RequiredResourcesPath != "" {
		errs = append(errs, errors.New("only one of 'requiredResources' or 'requiredResourcesPath' may be specified"))
	}

	return errs
}
