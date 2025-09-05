// Copyright 2025 Upbound Inc.
// All rights reserved

package v1alpha1

import "github.com/crossplane/crossplane-runtime/v2/pkg/errors"

// Hub marks this type as the conversion hub.
func (o *OperationTest) Hub() {}

// Convert converts []interface{} to []OperationTest, appending only valid OperationTest instances.
func Convert(parsedTests []interface{}) ([]OperationTest, error) {
	operationTests := make([]OperationTest, 0, len(parsedTests))
	var errs []error

	for _, t := range parsedTests {
		if test, ok := t.(OperationTest); ok {
			if err := test.Validate(); err != nil {
				errs = append(errs, err)
				continue // Skip invalid test
			}
			operationTests = append(operationTests, test)
		} // Silent skip if type assertion fails
	}

	// If no valid tests exist, return an error
	if len(operationTests) == 0 {
		if len(errs) > 0 {
			return nil, errors.Join(errs...)
		}
		return nil, errors.New("no valid OperationTests found")
	}

	// Return valid tests and any collected errors
	if len(errs) > 0 {
		return operationTests, errors.Join(errs...)
	}
	return operationTests, nil
}
