// Copyright 2025 Upbound Inc.
// All rights reserved

package v1alpha1

// Hub marks this type as the conversion hub.
func (p *E2ETest) Hub() {}

// Convert converts []interface{} to []E2ETest, appending only valid E2ETest instances.
func Convert(parsedTests []interface{}) ([]E2ETest, error) {
	e2eTests := make([]E2ETest, 0, len(parsedTests))

	for _, t := range parsedTests {
		if test, ok := t.(E2ETest); ok {
			e2eTests = append(e2eTests, test)
		} // Silent skip if type assertion fails
	}

	return e2eTests, nil
}
