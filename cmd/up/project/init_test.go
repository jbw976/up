// Copyright 2024 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package project contains commands for working with development projects.
package project

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

	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/internal/git"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/yaml"
	"github.com/upbound/up/pkg/apis/project/v1alpha1"
)

func TestAfterApply(t *testing.T) {
	type args struct {
		Flags     map[string]string
		Method    string
		Directory string
		SSHKey    string
		Name      string
	}

	tcs := map[string]struct {
		args        args
		expectError string
		expectedDir string
	}{
		"SSHWithoutKey": {
			args: args{
				Method: "ssh",
			},
			expectError: "SSH key must be specified when using SSH method",
		},
		"HTTPSWithKey": {
			args: args{
				Method: "https",
				SSHKey: "some-key",
			},
			expectError: "cannot specify SSH key when using HTTPS method",
		},
		"ValidSSH": {
			args: args{
				Method: "ssh",
				SSHKey: "valid-key",
				Name:   "test",
			},
			expectedDir: "test",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			// Prepare initCmd
			cmd := &initCmd{
				Flags:     upbound.Flags{},
				Method:    tc.args.Method,
				Directory: tc.args.Directory,
				SSHKey:    tc.args.SSHKey,
				Name:      tc.args.Name,
			}

			parser, err := kong.New(&struct{}{})
			assert.NilError(t, err)

			kongCtx, err := parser.Parse([]string{})
			assert.NilError(t, err)

			// Run AfterApply
			err = cmd.AfterApply(kongCtx)

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

//go:embed testdata/project-template
var projectTemplate embed.FS

// ManualMockGitCloner implements the git.Cloner interface.
type ManualMockGitCloner struct{}

func (m *ManualMockGitCloner) CloneRepository(_ storage.Storer, _ billy.Filesystem, _ git.AuthProvider, _ git.CloneOptions) (*plumbing.Reference, error) {
	return plumbing.NewHashReference("refs/heads/main", plumbing.NewHash("1234567890abcdef")), nil
}

func TestRun(t *testing.T) {
	type args struct {
		Flags        map[string]string
		Method       string
		Directory    string
		SSHKey       string
		Name         string
		Organization string
		Project      *v1alpha1.Project
	}

	tcs := map[string]struct {
		args          args
		expectedError string
	}{
		"Successful": {
			args: args{
				Method:       "ssh",
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
							Description: "This is where you can describe your project.",
							License:     "Apache-2.0",
							Maintainer:  "Upbound User <user@example.com>",
							Source:      "github.com/upbound/project-template",
							Readme:      "This is where you can add a readme for your project.\n",
						},
						Repository:    "donotuse.example.com/unit-test/test-project",
						Architectures: []string{"amd64", "arm64"},
						Paths: &v1alpha1.ProjectPaths{
							APIs:      "/apis",
							Examples:  "/examples",
							Functions: "/functions",
						},
					},
				},
			},
		},
		"OtherOrg": {
			args: args{
				Method:       "ssh",
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
							Description: "This is where you can describe your project.",
							License:     "Apache-2.0",
							Maintainer:  "Upbound User <user@example.com>",
							Source:      "github.com/upbound/project-template",
							Readme:      "This is where you can add a readme for your project.\n",
						},
						Repository:    "donotuse.example.com/up-test-org/test-project",
						Architectures: []string{"amd64", "arm64"},
						Paths: &v1alpha1.ProjectPaths{
							APIs:      "/apis",
							Examples:  "/examples",
							Functions: "/functions",
						},
					},
				},
			},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			tempProjDir, err := afero.TempDir(afero.NewOsFs(), os.TempDir(), "projFS")
			assert.NilError(t, err)
			defer os.RemoveAll(tempProjDir)

			srcFS := afero.NewBasePathFs(afero.FromIOFS{FS: projectTemplate}, "testdata/project-template")
			projFS := afero.NewBasePathFs(afero.NewOsFs(), tempProjDir)

			err = filesystem.CopyFilesBetweenFs(srcFS, projFS)
			assert.NilError(t, err)

			mockCloner := &ManualMockGitCloner{}

			cmd := &initCmd{
				Flags:           upbound.Flags{},
				Method:          tc.args.Method,
				SSHKey:          tc.args.SSHKey,
				Name:            tc.args.Name,
				projFile:        "upbound.yaml",
				projFS:          projFS,
				gitCloner:       mockCloner, // Pass the mock
				gitAuthProvider: nil,
			}

			ep, err := url.Parse("https://donotuse.example.com")
			assert.NilError(t, err)
			upCtx := &upbound.Context{
				Domain:           &url.URL{},
				RegistryEndpoint: ep,
				Organization:     tc.args.Organization,
			}

			err = cmd.Run(upCtx, &pterm.DefaultBasicText)
			assert.NilError(t, err)

			// **Read the file contents of upbound.yaml**
			filePath := tc.args.Directory + "/upbound.yaml"
			data, err := afero.ReadFile(projFS, filePath)
			if err != nil {
				t.Fatalf("error reading upbound.yaml: %v", err)
			}

			// **Unmarshal the file contents into a v1alpha1.Project**
			var parsedProject v1alpha1.Project
			err = yaml.Unmarshal(data, &parsedProject)
			if err != nil {
				t.Fatalf("error unmarshaling upbound.yaml into Project struct: %v", err)
			}

			// **Compare the unmarshaled project with the expected project**
			assert.DeepEqual(t, parsedProject, *tc.args.Project)
		})
	}
}
