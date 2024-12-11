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

package git

import (
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage"
	"github.com/go-git/go-git/v5/storage/memory"
	"gotest.tools/v3/assert"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// TestCloneRepository tests the CloneRepository.
func TestCloneRepository(t *testing.T) {
	type args struct {
		options     CloneOptions
		mockAuth    *MockAuthProvider
		mockCloner  *MockCloner
		expectError string
		expectRef   string
	}

	tcs := map[string]args{
		"ValidHTTPS": {
			options: CloneOptions{
				Repo:      "https://github.com/example/repo.git",
				RefName:   "refs/heads/main",
				Directory: "testdir",
			},
			mockAuth: &MockAuthProvider{
				GetAuthMethodFunc: func() (transport.AuthMethod, error) {
					return &http.BasicAuth{Username: "user", Password: "pass"}, nil
				},
			},
			mockCloner: &MockCloner{
				CloneRepositoryFunc: func(storer storage.Storer, _ billy.Filesystem, _ AuthProvider, _ CloneOptions) (*plumbing.Reference, error) {
					// Set the `refs/heads/main` reference
					mainRef := plumbing.NewHashReference(plumbing.ReferenceName("refs/heads/main"), plumbing.NewHash("mocksha"))
					if err := storer.SetReference(mainRef); err != nil {
						return nil, errors.Wrap(err, "failed to set reference in storer")
					}

					// Set the `HEAD` symbolic reference to point to `refs/heads/main`
					headRef := plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.ReferenceName("refs/heads/main"))
					if err := storer.SetReference(headRef); err != nil {
						return nil, errors.Wrap(err, "failed to set HEAD reference in storer")
					}

					// Return the symbolic reference for HEAD
					return headRef, nil
				},
			},
			expectRef: "refs/heads/main",
		},
		"CloneFailure": {
			options: CloneOptions{
				Repo:      "https://github.com/example/repo.git",
				RefName:   "main",
				Directory: "testdir",
			},
			mockAuth: &MockAuthProvider{
				GetAuthMethodFunc: func() (transport.AuthMethod, error) {
					return &http.BasicAuth{Username: "user", Password: "pass"}, nil
				},
			},
			mockCloner: &MockCloner{
				CloneRepositoryFunc: func(storage.Storer, billy.Filesystem, AuthProvider, CloneOptions) (*plumbing.Reference, error) {
					return nil, errors.New("failed to clone repository")
				},
			},
			expectError: "failed to clone repository",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			// Call CloneRepository using the mock Cloner
			ref, err := tc.mockCloner.CloneRepository(memory.NewStorage(), nil, tc.mockAuth, tc.options)

			// Validate results
			if tc.expectError != "" {
				assert.ErrorContains(t, err, tc.expectError)
			} else {
				assert.NilError(t, err)

				// Since HEAD is a symbolic reference, we should resolve it
				resolvedRef := ref.Target()
				assert.Equal(t, resolvedRef.String(), tc.expectRef)
			}
		})
	}
}

type MockAuthProvider struct {
	GetAuthMethodFunc func() (transport.AuthMethod, error)
}

func (m *MockAuthProvider) GetAuthMethod() (transport.AuthMethod, error) {
	if m.GetAuthMethodFunc != nil {
		return m.GetAuthMethodFunc()
	}
	return nil, nil
}

type MockCloner struct {
	CloneRepositoryFunc func(storage storage.Storer, fs billy.Filesystem, auth AuthProvider, opts CloneOptions) (*plumbing.Reference, error)
}

func (m *MockCloner) CloneRepository(storage storage.Storer, fs billy.Filesystem, auth AuthProvider, opts CloneOptions) (*plumbing.Reference, error) {
	if m.CloneRepositoryFunc != nil {
		return m.CloneRepositoryFunc(storage, fs, auth, opts)
	}
	return nil, nil
}
