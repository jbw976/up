// Copyright 2024 Upbound Inc
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

package project

import (
	"github.com/spf13/afero"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/pkg/apis/project/v1alpha1"
)

// Parse parses and validates the project file.
func Parse(projFS afero.Fs, projFilePath string) (*v1alpha1.Project, error) {
	// Parse and validate the project file.
	projYAML, err := afero.ReadFile(projFS, projFilePath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read project file %q", projFilePath)
	}
	var project v1alpha1.Project
	err = yaml.Unmarshal(projYAML, &project)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse project file")
	}
	if err := project.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid project file")
	}
	return &project, nil
}
