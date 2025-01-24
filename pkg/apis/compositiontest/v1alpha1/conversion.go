// Copyright 2025 Upbound Inc.
// All rights reserved

package v1alpha1

import "github.com/crossplane/crossplane-runtime/pkg/errors"

// Hub marks this type as the conversion hub.
func (c *CompositionTest) Hub() {}

// Convert converts []interface{} to []CompositionTest, appending only valid CompositionTest instances.
func Convert(parsedTests []interface{}) ([]CompositionTest, error) {
	compositionTests := make([]CompositionTest, 0, len(parsedTests))
	var errs []error

	for _, t := range parsedTests {
		if test, ok := t.(CompositionTest); ok {
			if err := test.Validate(); err != nil {
				errs = append(errs, err)
				continue // Skip invalid test
			}
			compositionTests = append(compositionTests, test)
		} // Silent skip if type assertion fails
	}

	// If no valid tests exist, return an error
	if len(compositionTests) == 0 {
		if len(errs) > 0 {
			return nil, errors.Join(errs...)
		}
		return nil, errors.New("no valid CompositionTests found")
	}

	// Return valid tests and any collected errors
	if len(errs) > 0 {
		return compositionTests, errors.Join(errs...)
	}
	return compositionTests, nil
}
