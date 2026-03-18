// Copyright 2025 Upbound Inc.
// All rights reserved

// Package test contains helpers for up project test
package test

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/spf13/afero"
	yamlv3 "gopkg.in/yaml.v3"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/upbound/up/internal/schemas/runner"
	"github.com/upbound/up/internal/yaml"
	compositionTest "github.com/upbound/up/pkg/apis/compositiontest/v1alpha1"
	e2etest "github.com/upbound/up/pkg/apis/e2etest/v1alpha1"
	operationtest "github.com/upbound/up/pkg/apis/operationtest/v1alpha1"
)

// Runner defines an interface for running a specific test type.
type Runner interface {
	Run(ctx context.Context, fs afero.Fs, basePath string, schemaRunner runner.SchemaRunner) error
}

// KCLRunner implements the TestType interface for KCL tests.
type KCLRunner struct{}

// Run kcl tests manifest generation.
func (t *KCLRunner) Run(ctx context.Context, fs afero.Fs, basePath string, schemaRunner runner.SchemaRunner) error {
	err := schemaRunner.Generate(
		ctx,
		fs,
		".",
		basePath,
		"xpkg.upbound.io/upbound/kcl:v0.11.2",
		[]string{"kcl", "run", "-o", "test.yaml"},
	)
	if err != nil {
		return errors.Wrap(err, "failed to execute KCL manifest generation")
	}
	return nil
}

// PythonRunner implements the TestType interface for Python tests.
type PythonRunner struct{}

// Run python tests manifest generation.
func (t *PythonRunner) Run(ctx context.Context, fs afero.Fs, basePath string, schemaRunner runner.SchemaRunner) error {
	err := schemaRunner.Generate(
		ctx,
		fs,
		".",
		basePath,
		"xpkg.upbound.io/upbound/uptest-pyrunner:v0.3.0",
		[]string{"/venv/test/bin/uptestpyrunner"},
		runner.WithWorkDirectory("/"),
		runner.WithCopyToPath("/venv/test/lib/python3.11/site-packages/uptestpyrunner"),
		runner.WithCopyFromPath("/test.yaml"),
	)
	if err != nil {
		return errors.Wrap(err, "failed to execute Python manifest generation")
	}
	return nil
}

// GoRunner implements the TestType interface for Go tests.
type GoRunner struct{}

// Run go tests manifest generation.
func (t *GoRunner) Run(ctx context.Context, fs afero.Fs, basePath string, _ runner.SchemaRunner) error {
	// Go tests run locally using "go run ." instead of in a container
	cmd := exec.CommandContext(ctx, "go", "run", ".")
	cmd.Dir = basePath

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "failed to execute 'go run .': %s", stderr.String())
	}

	// Write the output to test.yaml in the filesystem
	if err := afero.WriteFile(fs, "test.yaml", stdout.Bytes(), 0o644); err != nil {
		return errors.Wrap(err, "failed to write test.yaml")
	}

	return nil
}

// GoTemplatingRunner implements the TestType interface for go templating tests.
type GoTemplatingRunner struct{}

// Run go templating tests manifest generation.
func (t *GoTemplatingRunner) Run(_ context.Context, fs afero.Fs, _ string, _ runner.SchemaRunner) error {
	// We can use "*" as the pattern to parse because the GoTemplatingRunner is
	// selected only when all files in the directory end in .tmpl or .gotmpl.
	tmplData, err := readTemplates(fs)
	if err != nil {
		return errors.Wrap(err, "failed to read templates")
	}
	tmpl, err := template.New("").
		Funcs(sprig.FuncMap()).
		Parse(tmplData)
	if err != nil {
		return errors.Wrap(err, "failed to parse templates")
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, nil); err != nil {
		return errors.Wrapf(err, "failed to execute templates")
	}

	if err := afero.WriteFile(fs, "test.yaml", buf.Bytes(), 0o644); err != nil {
		return errors.Wrap(err, "failed to write test.yaml")
	}

	return nil
}

// YAMLRunner implements the TestType interface for raw YAML tests.
type YAMLRunner struct{}

// Run yaml tests - normalizes flexible YAML format to standard items format.
func (t *YAMLRunner) Run(_ context.Context, fs afero.Fs, _ string, _ runner.SchemaRunner) error {
	raw, err := afero.ReadFile(fs, "test.yaml")
	if err != nil {
		return errors.Wrap(err, "failed to read test.yaml")
	}

	// Decode multi-document YAML properly (handles --- with whitespace, leading separators, etc.)
	dec := yamlv3.NewDecoder(bytes.NewReader(raw))

	var docs []any
	for {
		var doc any
		if err := dec.Decode(&doc); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return errors.Wrap(err, "failed to decode YAML document")
		}

		// yaml decoder can return nil for empty docs
		if doc == nil {
			continue
		}
		docs = append(docs, doc)
	}

	if len(docs) == 0 {
		return errors.New("test.yaml is empty or contains only empty documents")
	}

	// If it's a single doc and already has items, we consider it normalized
	if len(docs) == 1 {
		if m, ok := asMap(docs[0]); ok {
			if _, hasItems := m["items"]; hasItems {
				return nil // Already normalized
			}
		}
	}

	// Collect items from docs
	var items []any
	for i, d := range docs {
		m, ok := asMap(d)
		if !ok {
			return errors.Errorf("unexpected YAML structure in document %d: expected mapping/object", i+1)
		}

		// Guard against mixed formats
		if _, hasItems := m["items"]; hasItems {
			if len(docs) > 1 {
				return errors.Errorf("document %d contains 'items'; mixed multi-doc + items format is not supported", i+1)
			}
			// Single doc with items - already handled above
			return nil
		}

		kind, _ := m["kind"].(string)
		switch kind {
		case "CompositionTest", "OperationTest", "E2ETest":
			items = append(items, m)
		case "":
			return errors.Errorf("document %d missing required 'kind' field", i+1)
		default:
			return errors.Errorf("unknown test kind in document %d: %s", i+1, kind)
		}
	}

	if len(items) == 0 {
		return errors.New("no test documents found to normalize")
	}

	// Write normalized output using internal yaml package for consistent formatting
	outObj := map[string]any{
		"items": items,
	}

	outBytes, err := yaml.Marshal(outObj)
	if err != nil {
		return errors.Wrap(err, "failed to marshal normalized YAML")
	}

	if err := afero.WriteFile(fs, "test.yaml", outBytes, 0o644); err != nil {
		return errors.Wrap(err, "failed to write normalized test.yaml")
	}

	return nil
}

// asMap converts various map types to map[string]interface{}.
func asMap(v any) (map[string]any, bool) {
	switch m := v.(type) {
	case map[string]any:
		return m, true
	case map[any]any:
		// yaml.v3 may return map[interface{}]interface{}
		// Convert to map[string]interface{} where possible
		out := make(map[string]any, len(m))
		for k, val := range m {
			ks, ok := k.(string)
			if !ok {
				return nil, false
			}
			out[ks] = val
		}
		return out, true
	default:
		return nil, false
	}
}

func readTemplates(fsys afero.Fs) (string, error) {
	const dotCharacter = 46

	tmpl := ""

	if err := afero.Walk(fsys, ".", func(path string, info fs.FileInfo, e error) error {
		if e != nil {
			return e
		}

		// check for directory and hidden files/folders
		if info.IsDir() || info.Name()[0] == dotCharacter {
			return nil
		}

		data, err := afero.ReadFile(fsys, path)
		if err != nil {
			return err
		}

		tmpl += string(data)
		tmpl += "\n---\n"

		return nil
	}); err != nil {
		return "", err
	}

	return tmpl, nil
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
	if exists, _ := afero.Exists(fs, "go.mod"); exists {
		return &GoRunner{}, nil
	}
	if containsGoTemplating(fs) {
		return &GoTemplatingRunner{}, nil
	}
	// Check for raw YAML tests - test.yaml file exists without any language-specific files
	if exists, _ := afero.Exists(fs, "test.yaml"); exists {
		return &YAMLRunner{}, nil
	}
	return nil, errors.New("no supported test type found")
}

func containsGoTemplating(tmplFS afero.Fs) bool {
	goTemplatingExtensions := []string{
		".gotmpl",
		".tmpl",
	}

	// The go templating builder will match any directory containing only files
	// with recognized extensions. Nested directories are allowed. An empty
	// directory is not matched.
	matches := false
	_ = afero.Walk(tmplFS, ".", func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Ignore directories but recurse into them.
		if info.Mode().IsDir() {
			return nil
		}

		// Don't support symlinks or any other funky stuff. We don't need to
		// symlink in models like we do for other languages, so it's simplest to
		// just not support them.
		if !info.Mode().IsRegular() {
			matches = false
			return fs.SkipAll
		}

		if !slices.Contains(goTemplatingExtensions, filepath.Ext(path)) {
			matches = false
			return fs.SkipAll
		}

		matches = true
		return nil
	})

	return matches
}

// Builder builds a project into test results.
type Builder interface {
	// Build dynamically identifies test types and runs them.
	Build(ctx context.Context, fs afero.Fs, patterns []string, testsFolder string, opts ...BuildOption) ([]any, error)
}

// BuildOption configures a build.
type BuildOption func(o *buildOptions)

// buildOptions holds configuration options for the build process.
type buildOptions struct {
	schemaRunner   runner.SchemaRunner
	testIdentifier Identifier
}

// BuildWithSchemaRunner sets the schema runner.
func BuildWithSchemaRunner(r runner.SchemaRunner) BuildOption {
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
func (b *realBuilder) Build(ctx context.Context, fs afero.Fs, patterns []string, testsFolder string, opts ...BuildOption) ([]any, error) { //nolint:gocognit // building and cast tests
	buildOpts := *b.options
	for _, opt := range opts {
		opt(&buildOpts)
	}

	testDirs, err := discoverTestDirectories(fs, patterns, testsFolder)
	if err != nil {
		return nil, errors.Wrap(err, "failed to discover test directories")
	}

	var results []any
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

		var rawContent map[string]any
		if err := yaml.Unmarshal(resourceRaw, &rawContent); err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal test.yaml in %q", testDir)
		}

		items, ok := rawContent["items"].([]any)
		if !ok {
			return nil, errors.Errorf("expected `items` array in test.yaml in %q", testDir)
		}

		for _, item := range items {
			switch item.(type) {
			case map[string]any, map[any]any, struct{}:
				// Continue to marshal
			default:
				continue
			}

			itemBytes, err := yaml.Marshal(item)
			if err != nil {
				continue
			}

			var testObj any
			itemMap, ok := item.(map[string]any)
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
				case "OperationTest":
					var opTest operationtest.OperationTest
					if err := yaml.Unmarshal(itemBytes, &opTest); err != nil {
						continue
					}
					testObj = opTest
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

func discoverTestDirectories(fs afero.Fs, patterns []string, testsFolder string) ([]string, error) {
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
