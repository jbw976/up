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
