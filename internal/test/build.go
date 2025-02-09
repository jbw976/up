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

// Package test contains helpers for up project test
package test

import (
	"context"
	"strings"

	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/xpkg/schemarunner"
	"github.com/upbound/up/internal/yaml"
	compositionTest "github.com/upbound/up/pkg/apis/compositiontest/v1alpha1"
	e2etest "github.com/upbound/up/pkg/apis/e2etest/v1alpha1"
)

// Runner defines an interface for running a specific test type.
type Runner interface {
	Run(ctx context.Context, fs afero.Fs, basePath string, schemaRunner schemarunner.SchemaRunner) error
}

// KCLRunner implements the TestType interface for KCL tests.
type KCLRunner struct{}

// Run kcl tests manifest generation.
func (t *KCLRunner) Run(ctx context.Context, fs afero.Fs, basePath string, schemaRunner schemarunner.SchemaRunner) error {
	err := schemaRunner.Generate(ctx, fs, ".", basePath, "xpkg.upbound.io/upbound/kcl:v0.10.6", []string{"kcl", "run", "-o", "test.yaml"})
	if err != nil {
		return errors.Wrap(err, "failed to execute KCL manifest generation")
	}
	return nil
}

// PythonRunner implements the TestType interface for Python tests.
type PythonRunner struct{}

// Run python tests manifest generation.
func (t *PythonRunner) Run(ctx context.Context, fs afero.Fs, basePath string, schemaRunner schemarunner.SchemaRunner) error {
	// ToDo(haarchri): switch to upbound
	err := schemaRunner.Generate(ctx, fs, ".", basePath, "docker.io/haarchri/python-test:0.9", []string{"python", "-m", "extract_objects"})
	if err != nil {
		return errors.Wrap(err, "failed to execute Python manifest generation")
	}
	return nil
}

// Identifier determines the appropriate TestType based on the filesystem.
type Identifier interface {
	Identify(fs afero.Fs) (Runner, error)
}

// DefaultIdentifier is the default implementation of TestIdentifier.
type DefaultIdentifier struct{}

// Identify is trying to identitfy supported languages.
func (i *DefaultIdentifier) Identify(fs afero.Fs) (Runner, error) {
	if exists, _ := afero.Exists(fs, "kcl.mod"); exists {
		return &KCLRunner{}, nil
	}
	if exists, _ := afero.Exists(fs, "main.py"); exists {
		return &PythonRunner{}, nil
	}
	return nil, errors.New("no supported test type found")
}

// Builder builds a project into test results.
type Builder interface {
	// Build dynamically identifies test types and runs them.
	Build(ctx context.Context, fs afero.Fs, patterns []string, testsFolder, testPrefix string, opts ...BuildOption) ([]interface{}, error)
}

// BuildOption configures a build.
type BuildOption func(o *buildOptions)

// buildOptions holds configuration options for the build process.
type buildOptions struct {
	schemaRunner   schemarunner.SchemaRunner
	testIdentifier Identifier
}

// BuildWithSchemaRunner sets the schema runner.
func BuildWithSchemaRunner(r schemarunner.SchemaRunner) BuildOption {
	return func(o *buildOptions) {
		o.schemaRunner = r
	}
}

// BuildWithTestIdentifier sets the test identifier.
func BuildWithTestIdentifier(identifier Identifier) BuildOption {
	return func(o *buildOptions) {
		o.testIdentifier = identifier
	}
}

// realBuilder is the concrete implementation of the Builder interface.
type realBuilder struct {
	options *buildOptions
}

// NewBuilder creates a new Builder instance with the provided options.
func NewBuilder(opts ...BuildOption) Builder {
	options := &buildOptions{
		testIdentifier: &DefaultIdentifier{}, // Default identifier
	}
	for _, opt := range opts {
		opt(options)
	}
	return &realBuilder{options: options}
}

// Build implements the Builder interface to identify and run tests.
func (b *realBuilder) Build(ctx context.Context, fs afero.Fs, patterns []string, testsFolder string, testPrefix string, opts ...BuildOption) ([]interface{}, error) { //nolint:gocognit // building and cast tests
	buildOpts := *b.options
	for _, opt := range opts {
		opt(&buildOpts)
	}

	testDirs, err := discoverTestDirectories(fs, patterns, testsFolder, testPrefix)
	if err != nil {
		return nil, errors.Wrap(err, "failed to discover test directories")
	}

	var results []interface{}
	for _, testDir := range testDirs {
		testFS := afero.NewBasePathFs(fs, testDir)
		langType, err := buildOpts.testIdentifier.Identify(testFS)
		if err != nil {
			continue
		}

		basePath := ""
		if bfs, ok := testFS.(*afero.BasePathFs); ok && basePath == "" {
			basePath = afero.FullBaseFsPath(bfs, ".")
		}

		testFS = afero.NewCopyOnWriteFs(testFS, afero.NewMemMapFs())
		err = langType.Run(ctx, testFS, basePath, buildOpts.schemaRunner)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to run test in %q", testDir)
		}

		resourceRaw, err := afero.ReadFile(testFS, "test.yaml")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read test.yaml in %q", testDir)
		}

		var rawContent map[string]interface{}
		if err := yaml.Unmarshal(resourceRaw, &rawContent); err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal test.yaml in %q", testDir)
		}

		items, ok := rawContent["items"].([]interface{})
		if !ok {
			return nil, errors.Errorf("expected `items` array in test.yaml in %q", testDir)
		}

		for _, item := range items {
			itemBytes, err := yaml.Marshal(item)
			if err != nil {
				continue
			}

			var testObj interface{}
			itemMap, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			if kind, exists := itemMap["kind"].(string); exists {
				switch kind {
				case "E2ETest":
					var e2eTest e2etest.E2ETest
					if err := yaml.Unmarshal(itemBytes, &e2eTest); err != nil {
						continue
					}
					testObj = e2eTest
				case "CompositionTest":
					var compTest compositionTest.CompositionTest
					if err := yaml.Unmarshal(itemBytes, &compTest); err != nil {
						continue
					}
					testObj = compTest
				default:
					continue
				}
			} else {
				continue
			}

			results = append(results, testObj)
		}
	}
	return results, nil
}

func discoverTestDirectories(fs afero.Fs, patterns []string, testsFolder string, testPrefix string) ([]string, error) {
	var matchedDirs []string

	cleanedPatterns := make([]string, len(patterns))
	for i, pattern := range patterns {
		trimmed := strings.TrimPrefix(pattern, testsFolder)
		if trimmed == "" {
			trimmed = "*" // Match all subdirectories if testsFolder was the full pattern
		}
		cleanedPatterns[i] = trimmed
	}

	for _, pattern := range cleanedPatterns {
		matches, err := afero.Glob(fs, pattern)
		if err != nil {
			return nil, err
		}

		for _, match := range matches {
			if !strings.HasPrefix(match, testPrefix) {
				continue
			}
			isDir, err := afero.IsDir(fs, match)
			if err != nil {
				return nil, err
			}
			if isDir {
				matchedDirs = append(matchedDirs, match)
			}
		}
	}

	return matchedDirs, nil
}
