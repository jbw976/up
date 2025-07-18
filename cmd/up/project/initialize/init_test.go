// Copyright 2025 Upbound Inc.
// All rights reserved

// Package project contains commands for working with development projects.
package initialize

import (
	"embed"
	"net/url"
	"os"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage"
	"github.com/pterm/pterm"
	"github.com/spf13/afero"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"

	"github.com/upbound/up/cmd/up/runner"
	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/internal/git"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/yaml"
	"github.com/upbound/up/pkg/apis/project/v1alpha1"
)

var errBoom = errors.New("boom")

type mockCommandRunner struct{}

func (m *mockCommandRunner) RunCommand(_ []string) error {
	return nil
}

var _ runner.CommandRunner = &mockCommandRunner{}

func TestAfterApply(t *testing.T) {
	type args struct {
		Flags     map[string]string
		Directory string
		SSHKey    string
		Name      string
		Template  string
	}

	tcs := map[string]struct {
		args        args
		expectError string
		expectedDir string
	}{
		"SSHWithoutKey": {
			args: args{
				Template: "ssh://user@host/path/to/repo",
			},
			expectError: "SSH key must be specified when using SSH protocol",
		},
		"HTTPSWithKey": {
			args: args{
				Template: "https://path/to/repo",
				SSHKey:   "some-key",
			},
			expectError: "cannot specify SSH key when using HTTPS protocol",
		},
		"UnsupportedProtocol": {
			args: args{
				Template: "git://path/to/repo",
			},
			expectError: "unsupported protocol git in template url",
		},
		"MissingLanguage": {
			args: args{
				Template: "ssh://user@host/path/to/repo",
				SSHKey:   "valid-key",
				Name:     "test",
			},
			expectError: "language must be specified when using a template",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			// Prepare Cmd
			cmd := &Cmd{
				Flags:     upbound.Flags{},
				Directory: tc.args.Directory,
				SSHKey:    tc.args.SSHKey,
				Name:      tc.args.Name,
				Template:  tc.args.Template,
			}

			parser, err := kong.New(&struct{}{})
			assert.NilError(t, err)

			kongCtx, err := parser.Parse([]string{})
			assert.NilError(t, err)

			// Run AfterApply
			err = cmd.AfterApply(kongCtx, &mockCommandRunner{})

			// Validate error
			if tc.expectError != "" {
				assert.ErrorContains(t, err, tc.expectError)
			} else {
				assert.NilError(t, err)
			}

			// Validate directory setting
			if tc.expectedDir != "" {
				assert.Equal(t, cmd.Directory, tc.expectedDir)
			}
		})
	}
}

//go:embed testdata/example-project
var exampleProject embed.FS

//go:embed testdata/example-scratch
var exampleScratch embed.FS

// ManualMockGitCloner implements the git.Cloner interface.
type ManualMockGitCloner struct{}

func (m *ManualMockGitCloner) CloneRepository(_ storage.Storer, _ billy.Filesystem, _ git.AuthProvider, _ git.CloneOptions) (*plumbing.Reference, error) {
	return plumbing.NewHashReference("refs/heads/main", plumbing.NewHash("1234567890abcdef")), nil
}

type CopyMockGitCloner struct {
	CopyFunc func(storer storage.Storer, fs billy.Filesystem, authProvider git.AuthProvider, opts git.CloneOptions) error
}

func (m *CopyMockGitCloner) CloneRepository(storer storage.Storer, fs billy.Filesystem, authProvider git.AuthProvider, opts git.CloneOptions) (*plumbing.Reference, error) {
	if m.CopyFunc != nil {
		err := m.CopyFunc(storer, fs, authProvider, opts)
		if err != nil {
			return nil, err
		}
	}

	return plumbing.NewHashReference("refs/heads/main", plumbing.NewHash("1234567890abcdef")), nil
}

func TestRun_Scratch(t *testing.T) {
	srcFS := afero.NewBasePathFs(afero.FromIOFS{FS: exampleScratch}, "testdata/example-scratch")

	cloneProject := &CopyMockGitCloner{
		CopyFunc: func(_ storage.Storer, fs billy.Filesystem, _ git.AuthProvider, _ git.CloneOptions) error {
			// Create the target directory
			if err := fs.MkdirAll(fs.Root(), 0o755); err != nil {
				return err
			}
			tempCloneDir := afero.NewBasePathFs(afero.NewOsFs(), fs.Root())
			return filesystem.CopyFilesBetweenFs(srcFS, tempCloneDir)
		},
	}

	type args struct {
		SSHKey       string
		Name         string
		Organization string
		Project      *v1alpha1.Project
	}

	tcs := map[string]struct {
		args          args
		expectedError string
		mockCloner    git.Cloner
	}{
		"Successful": {
			args: args{
				Name:         "test-project",
				Organization: "unit-test",
				Project: &v1alpha1.Project{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "meta.dev.upbound.io/v1alpha1",
						Kind:       "Project",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-project",
					},
					Spec: &v1alpha1.ProjectSpec{
						ProjectPackageMetadata: v1alpha1.ProjectPackageMetadata{
							Description: "A template for unit testing project examples",
							License:     "Apache-2.0",
							Maintainer:  "Upbound User <user@example.com>",
							Source:      "github.com/upbound/unit-test-example",
							Readme:      "",
						},
						Repository: "donotuse.example.com/unit-test/test-project",
					},
				},
			},
			mockCloner: cloneProject,
		},
		"OtherOrg": {
			args: args{
				Name:         "test-project",
				Organization: "up-test-org",
				Project: &v1alpha1.Project{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "meta.dev.upbound.io/v1alpha1",
						Kind:       "Project",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-project",
					},
					Spec: &v1alpha1.ProjectSpec{
						ProjectPackageMetadata: v1alpha1.ProjectPackageMetadata{
							Description: "A template for unit testing project examples",
							License:     "Apache-2.0",
							Maintainer:  "Upbound User <user@example.com>",
							Source:      "github.com/upbound/unit-test-example",
							Readme:      "",
						},
						Repository: "donotuse.example.com/up-test-org/test-project",
					},
				},
			},
			mockCloner: cloneProject,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			tempProjDir, err := afero.TempDir(afero.NewOsFs(), os.TempDir(), "projFS")
			assert.NilError(t, err)
			defer os.RemoveAll(tempProjDir)

			projFS := afero.NewBasePathFs(afero.NewOsFs(), tempProjDir)

			cmd := &Cmd{
				Flags:           upbound.Flags{},
				Scratch:         true,
				Language:        "kcl",
				TestLanguage:    "kcl",
				SSHKey:          tc.args.SSHKey,
				Name:            tc.args.Name,
				Directory:       tempProjDir,
				projFile:        "upbound.yaml",
				projFS:          projFS,
				gitCloner:       tc.mockCloner,
				gitAuthProvider: nil,
			}

			ep, err := url.Parse("https://donotuse.example.com")
			assert.NilError(t, err)
			upCtx := &upbound.Context{
				Domain:           &url.URL{},
				RegistryEndpoint: ep,
				Organization:     tc.args.Organization,
			}

			err = cmd.Run(t.Context(), upCtx, &pterm.DefaultBasicText)
			if tc.expectedError != "" {
				assert.ErrorContains(t, err, tc.expectedError)
				return
			}
			assert.NilError(t, err)

			// Verify the project file exists
			projectFile, err := afero.ReadFile(projFS, "./upbound.yaml")
			assert.NilError(t, err)

			var parsedProject v1alpha1.Project
			err = yaml.Unmarshal(projectFile, &parsedProject)
			assert.NilError(t, err)

			assert.DeepEqual(t, parsedProject, *tc.args.Project)
		})
	}
}

func TestRun_Example(t *testing.T) {
	srcFS := afero.NewBasePathFs(afero.FromIOFS{FS: exampleProject}, "testdata/example-project")

	cloneProject := &CopyMockGitCloner{
		CopyFunc: func(_ storage.Storer, fs billy.Filesystem, _ git.AuthProvider, _ git.CloneOptions) error {
			// Create the target directory
			if err := fs.MkdirAll(fs.Root(), 0o755); err != nil {
				return err
			}
			tempCloneDir := afero.NewBasePathFs(afero.NewOsFs(), fs.Root())
			return filesystem.CopyFilesBetweenFs(srcFS, tempCloneDir)
		},
	}

	type fileAssertionFunc func(t *testing.T, projFS afero.Fs)

	type args struct {
		Flags        map[string]string
		SSHKey       string
		Name         string
		Organization string
		Template     string
		Language     string
		TestLanguage string
		Values       map[string]string
		Project      *v1alpha1.Project
	}

	tcs := map[string]struct {
		args           args
		expectedError  string
		mockCloner     git.Cloner
		fileAssertions []fileAssertionFunc
	}{
		"SuccessfulExampleGo": {
			args: args{
				Name:         "test-project",
				Organization: "unit-test",
				Template:     "example-1",
				Language:     "go",
				TestLanguage: "go",
				Project: &v1alpha1.Project{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "meta.dev.upbound.io/v1alpha1",
						Kind:       "Project",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-project",
					},
					Spec: &v1alpha1.ProjectSpec{
						ProjectPackageMetadata: v1alpha1.ProjectPackageMetadata{
							Description: "A template for unit testing project examples",
							License:     "Apache-2.0",
							Maintainer:  "Upbound User <user@example.com>",
							Source:      "github.com/upbound/unit-test-example",
							Readme:      "",
						},
						Repository: "donotuse.example.com/unit-test/test-project",
					},
				},
			},
			mockCloner: cloneProject,
			fileAssertions: []fileAssertionFunc{
				func(t *testing.T, projFS afero.Fs) {
					contents, err := afero.ReadFile(projFS, "./apis/api/composition.yaml")
					assert.NilError(t, err)

					var parsedComposition v1.Composition
					err = yaml.Unmarshal(contents, &parsedComposition)
					assert.NilError(t, err)

					assert.Equal(t, parsedComposition.Spec.CompositeTypeRef.Kind, "XStorageBucket")
					assert.Equal(t, parsedComposition.Spec.Pipeline[0].FunctionRef.Name, "unit-test-test-projectfunction")
					assert.Equal(t, parsedComposition.Spec.Pipeline[1].FunctionRef.Name, "crossplane-contrib-function-auto-ready")
				},
			},
		},
		"SuccessfulExamplePython": {
			args: args{
				Name:         "test-project",
				Organization: "unit-test",
				Template:     "example-1",
				Language:     "python",
				TestLanguage: "python",
				Project: &v1alpha1.Project{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "meta.dev.upbound.io/v1alpha1",
						Kind:       "Project",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-project",
					},
					Spec: &v1alpha1.ProjectSpec{
						ProjectPackageMetadata: v1alpha1.ProjectPackageMetadata{
							Description: "A template for unit testing project examples",
							License:     "Apache-2.0",
							Maintainer:  "Upbound User <user@example.com>",
							Source:      "github.com/upbound/unit-test-example",
							Readme:      "",
						},
						Repository: "donotuse.example.com/unit-test/test-project",
					},
				},
			},
			mockCloner: cloneProject,
		},
		"DifferentTestLanguage": {
			args: args{
				Name:         "test-project",
				Organization: "unit-test",
				Template:     "example-1",
				Language:     "python",
				TestLanguage: "go",
				Project: &v1alpha1.Project{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "meta.dev.upbound.io/v1alpha1",
						Kind:       "Project",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-project",
					},
					Spec: &v1alpha1.ProjectSpec{
						ProjectPackageMetadata: v1alpha1.ProjectPackageMetadata{
							Description: "A template for unit testing project examples",
							License:     "Apache-2.0",
							Maintainer:  "Upbound User <user@example.com>",
							Source:      "github.com/upbound/unit-test-example",
							Readme:      "",
						},
						Repository: "donotuse.example.com/unit-test/test-project",
					},
				},
			},
			mockCloner: cloneProject,
		},
		"ProvidedValues": {
			args: args{
				Name:         "test-project",
				Organization: "unit-test",
				Template:     "example-1",
				Language:     "python",
				TestLanguage: "python",
				Values: map[string]string{
					"UserValue":       "user-value",
					"OverriddenValue": "overridden-value",
				},
				Project: &v1alpha1.Project{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "meta.dev.upbound.io/v1alpha1",
						Kind:       "Project",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-project",
					},
					Spec: &v1alpha1.ProjectSpec{
						ProjectPackageMetadata: v1alpha1.ProjectPackageMetadata{
							Description: "A template for unit testing project examples",
							License:     "Apache-2.0",
							Maintainer:  "Upbound User <user@example.com>",
							Source:      "github.com/upbound/unit-test-example",
							Readme:      "",
						},
						Repository: "donotuse.example.com/unit-test/test-project",
					},
				},
			},
			mockCloner: cloneProject,
			fileAssertions: []fileAssertionFunc{
				func(t *testing.T, projFS afero.Fs) {
					contents, err := afero.ReadFile(projFS, "./docs/user-value.md")
					assert.NilError(t, err)
					assert.Equal(t, string(contents), "user-value")

					contents, err = afero.ReadFile(projFS, "./docs/default-value.md")
					assert.NilError(t, err)
					assert.Equal(t, string(contents), "default-value")

					contents, err = afero.ReadFile(projFS, "./docs/overridden-value.md")
					assert.NilError(t, err)
					assert.Equal(t, string(contents), "overridden-value")
				},
			},
		},
		"InvalidExample": {
			args: args{
				Name:         "test-project",
				Organization: "unit-test",
				Template:     "non-existent-example",
			},
			expectedError: "failed to clone repository: boom",
			mockCloner: &CopyMockGitCloner{
				CopyFunc: func(_ storage.Storer, _ billy.Filesystem, _ git.AuthProvider, _ git.CloneOptions) error {
					return errBoom
				},
			},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			tempProjDir, err := afero.TempDir(afero.NewOsFs(), os.TempDir(), "projFS")
			assert.NilError(t, err)
			defer os.RemoveAll(tempProjDir)

			projFS := afero.NewBasePathFs(afero.NewOsFs(), tempProjDir)

			cmd := &Cmd{
				Flags:           upbound.Flags{},
				SSHKey:          tc.args.SSHKey,
				Name:            tc.args.Name,
				Template:        tc.args.Template,
				Language:        tc.args.Language,
				TestLanguage:    tc.args.TestLanguage,
				Values:          tc.args.Values,
				Directory:       tempProjDir,
				projFile:        "upbound.yaml",
				projFS:          projFS,
				gitCloner:       tc.mockCloner,
				gitAuthProvider: nil,
			}

			ep, err := url.Parse("https://donotuse.example.com")
			assert.NilError(t, err)
			upCtx := &upbound.Context{
				Domain:           &url.URL{},
				RegistryEndpoint: ep,
				Organization:     tc.args.Organization,
			}

			err = cmd.Run(t.Context(), upCtx, &pterm.DefaultBasicText)
			if tc.expectedError != "" {
				assert.ErrorContains(t, err, tc.expectedError)
				return
			}
			assert.NilError(t, err)

			// Verify the project file exists
			projectFile, err := afero.ReadFile(projFS, "./upbound.yaml")
			assert.NilError(t, err)

			var parsedProject v1alpha1.Project
			err = yaml.Unmarshal(projectFile, &parsedProject)
			assert.NilError(t, err)

			testLanguage := tc.args.TestLanguage
			if testLanguage == "" {
				testLanguage = tc.args.Language
			}

			// Verify the language-specific files exist
			langFunctionFile := "./functions/function/function." + tc.args.Language
			langExampleFile := "./examples/example/example." + tc.args.Language
			langAPIsFile := "./apis/api/api." + tc.args.Language
			langTestFile := "./tests/test/test." + testLanguage

			exists, err := afero.Exists(projFS, langFunctionFile)
			assert.NilError(t, err)
			assert.Equal(t, exists, true)
			exists, err = afero.Exists(projFS, langExampleFile)
			assert.NilError(t, err)
			assert.Equal(t, exists, true)
			exists, err = afero.Exists(projFS, langAPIsFile)
			assert.NilError(t, err)
			assert.Equal(t, exists, true)
			exists, err = afero.Exists(projFS, langTestFile)
			assert.NilError(t, err)
			assert.Equal(t, exists, true)

			// Verify other languages don't exist
			exists, err = afero.Exists(projFS, "./functions/function/function.rust")
			assert.NilError(t, err)
			assert.Equal(t, exists, false)
			exists, err = afero.Exists(projFS, "./examples/example/example.rust")
			assert.NilError(t, err)
			assert.Equal(t, exists, false)
			exists, err = afero.Exists(projFS, "./apis/api/api.rust")
			assert.NilError(t, err)
			assert.Equal(t, exists, false)
			exists, err = afero.Exists(projFS, "./tests/test/test.rust")
			assert.NilError(t, err)
			assert.Equal(t, exists, false)

			for _, fileAssertion := range tc.fileAssertions {
				fileAssertion(t, projFS)
			}

			assert.DeepEqual(t, parsedProject, *tc.args.Project)
		})
	}
}
