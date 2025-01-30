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

// Package test handling meta api test resource functions
package test

import (
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/yaml"
	"github.com/upbound/up/pkg/apis/e2etest/v1alpha1"
)

// ParseE2E parses the e2etest file, returning the parsed E2ETest resource.
func ParseE2E(projFS afero.Fs, e2eTestPath string) (*v1alpha1.E2ETest, error) {
	// Parse and validate the e2etest file.
	testYAML, err := afero.ReadFile(projFS, e2eTestPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read e2etest file %q", e2eTestPath)
	}
	var e2etest v1alpha1.E2ETest
	err = yaml.Unmarshal(testYAML, &e2etest)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse e2etest file")
	}
	// if err := project.Validate(); err != nil {
	// 	return nil, errors.Wrap(err, "invalid e2etest file")
	// }

	return &e2etest, nil
}
