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

import "fmt"

// Hub marks this type as the conversion hub.
func (p *E2ETest) Hub() {}

// Convert converts []interface{} to []E2ETest.
func Convert(parsedTests []interface{}) ([]E2ETest, error) {
	e2eTests := make([]E2ETest, 0, len(parsedTests))

	for _, t := range parsedTests {
		test, ok := t.(E2ETest)
		if !ok {
			return nil, fmt.Errorf("invalid type: expected E2ETest but got %T", t)
		}
		e2eTests = append(e2eTests, test)
	}

	return e2eTests, nil
}
