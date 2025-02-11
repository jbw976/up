// Copyright 2025 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
