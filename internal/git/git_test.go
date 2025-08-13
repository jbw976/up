// Copyright 2025 Upbound Inc.
// All rights reserved

package git

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage"
	"github.com/go-git/go-git/v5/storage/memory"
	"gotest.tools/v3/assert"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
)

// TestCloneRepository tests the CloneRepository.
func TestCloneRepository(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		options     CloneOptions
		mockAuth    *MockAuthProvider
		mockCloner  *MockCloner
		wantError   bool
		expectError string
		expectRef   string
	}{
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
			wantError: false,
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
			wantError:   true,
			expectError: "failed to clone repository",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Call CloneRepository using the mock Cloner
			ref, err := tc.mockCloner.CloneRepository(memory.NewStorage(), nil, tc.mockAuth, tc.options)

			// Validate results
			if tc.wantError {
				assert.Assert(t, err != nil)
				assert.ErrorContains(t, err, tc.expectError)
				return
			}

			assert.NilError(t, err)

			// Since HEAD is a symbolic reference, we should resolve it
			resolvedRef := ref.Target()
			assert.Equal(t, resolvedRef.String(), tc.expectRef)
		})
	}
}

// TestHTTPSAuthProvider tests the HTTPSAuthProvider.
func TestHTTPSAuthProvider(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		username string
		password string
		wantAuth bool
	}{
		"WithUsernameAndPassword": {
			username: "testuser",
			password: "testpass",
			wantAuth: true,
		},
		"WithUsernameOnly": {
			username: "testuser",
			password: "",
			wantAuth: true,
		},
		"WithPasswordOnly": {
			username: "",
			password: "testpass",
			wantAuth: true,
		},
		"WithoutCredentials": {
			username: "",
			password: "",
			wantAuth: false,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			provider := &HTTPSAuthProvider{
				Username: tc.username,
				Password: tc.password,
			}

			auth, err := provider.GetAuthMethod()
			assert.NilError(t, err)

			if tc.wantAuth {
				assert.Assert(t, auth != nil)
				httpAuth, ok := auth.(*http.BasicAuth)
				assert.Assert(t, ok)
				assert.Equal(t, httpAuth.Username, tc.username)
				assert.Equal(t, httpAuth.Password, tc.password)
			} else {
				assert.Assert(t, auth == nil)
			}
		})
	}
}

// TestSSHAuthProvider tests the SSHAuthProvider.
func TestSSHAuthProvider(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		setupKeyFile func(t *testing.T) string
		username     string
		passphrase   string
		wantError    bool
		expectedUser string
	}{
		"WithUsernameAndValidKey": {
			setupKeyFile: func(t *testing.T) string {
				tmpDir := t.TempDir()
				keyPath := filepath.Join(tmpDir, "test_key")
				keyContent := `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAFwAAAAdzc2gtcn
NhAAAAAwEAAQAAAQEAtest_key_content_here
-----END OPENSSH PRIVATE KEY-----`
				err := os.WriteFile(keyPath, []byte(keyContent), 0o600)
				assert.NilError(t, err)
				return keyPath
			},
			username:     "testuser",
			passphrase:   "",
			wantError:    true, // This will fail with the test key, which is expected
			expectedUser: "testuser",
		},
		"WithoutUsernameDefaultsToGit": {
			setupKeyFile: func(t *testing.T) string {
				tmpDir := t.TempDir()
				keyPath := filepath.Join(tmpDir, "test_key")
				keyContent := `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAFwAAAAdzc2gtcn
NhAAAAAwEAAQAAAQEAtest_key_content_here
-----END OPENSSH PRIVATE KEY-----`
				err := os.WriteFile(keyPath, []byte(keyContent), 0o600)
				assert.NilError(t, err)
				return keyPath
			},
			username:     "",
			passphrase:   "",
			wantError:    true, // This will fail with the test key, which is expected
			expectedUser: "git",
		},
		"WithInvalidKeyPath": {
			setupKeyFile: func(_ *testing.T) string {
				return "/non/existent/path"
			},
			username:     "testuser",
			passphrase:   "",
			wantError:    true,
			expectedUser: "testuser",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			keyPath := tc.setupKeyFile(t)
			provider := &SSHAuthProvider{
				Username:       tc.username,
				PrivateKeyPath: keyPath,
				Passphrase:     tc.passphrase,
			}

			auth, err := provider.GetAuthMethod()

			if tc.wantError {
				assert.Assert(t, err != nil)
				assert.Assert(t, auth == nil)
			} else {
				assert.NilError(t, err)
				assert.Assert(t, auth != nil)
			}
		})
	}
}

// TestCheckSHA256Hash tests the CheckSHA256Hash function.
func TestCheckSHA256Hash(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		input    string
		expected bool
	}{
		"ValidSHA": {
			input:    "a1b2c3d4e5f6789012345678901234567890abcd",
			expected: true,
		},
		"ValidSHAUppercase": {
			input:    "A1B2C3D4E5F6789012345678901234567890ABCD",
			expected: true,
		},
		"ValidSHAMixed": {
			input:    "a1B2c3D4e5F6789012345678901234567890AbCd",
			expected: true,
		},
		"TooShort": {
			input:    "a1b2c3d4e5f6789012345678901234567890abc",
			expected: false,
		},
		"TooLong": {
			input:    "a1b2c3d4e5f6789012345678901234567890abcde",
			expected: false,
		},
		"EmptyString": {
			input:    "",
			expected: false,
		},
		"ContainsInvalidCharacters": {
			input:    "a1b2c3d4e5f6789012345678901234567890abcg",
			expected: false,
		},
		"ContainsSpecialCharacters": {
			input:    "a1b2c3d4e5f6789012345678901234567890ab-d",
			expected: false,
		},
		"AllZeros": {
			input:    "0000000000000000000000000000000000000000",
			expected: true,
		},
		"AllNines": {
			input:    "9999999999999999999999999999999999999999",
			expected: true,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			result := CheckSHA256Hash(tc.input)
			assert.Equal(t, result, tc.expected)
		})
	}
}

// TestExtractBranchName tests the extractBranchName function.
func TestExtractBranchName(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		input    string
		expected string
	}{
		"FullRef": {
			input:    "refs/heads/main",
			expected: "main",
		},
		"FullRefFeatureBranch": {
			input:    "refs/heads/feature/test",
			expected: "feature/test",
		},
		"BranchNameOnly": {
			input:    "develop",
			expected: "develop",
		},
		"EmptyString": {
			input:    "",
			expected: "main",
		},
		"TagRef": {
			input:    "refs/tags/v1.0.0",
			expected: "main",
		},
		"RemoteRef": {
			input:    "refs/remotes/origin/main",
			expected: "main",
		},
		"JustRefs": {
			input:    "refs/",
			expected: "main",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			result := extractBranchName(tc.input)
			assert.Equal(t, result, tc.expected)
		})
	}
}

// TestDefaultCloner_CloneRepository tests the DefaultCloner.CloneRepository method.
func TestDefaultCloner_CloneRepository(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		setupAuthProvider func() AuthProvider
		options           CloneOptions
		wantError         bool
		expectError       string
	}{
		"AuthProviderError": {
			setupAuthProvider: func() AuthProvider {
				return &MockAuthProvider{
					GetAuthMethodFunc: func() (transport.AuthMethod, error) {
						return nil, errors.New("auth provider error")
					},
				}
			},
			options: CloneOptions{
				Repo:    "https://github.com/test/repo.git",
				RefName: "main",
			},
			wantError:   true,
			expectError: "failed to get authentication method",
		},
		"InvalidRepo": {
			setupAuthProvider: func() AuthProvider {
				return &MockAuthProvider{
					GetAuthMethodFunc: func() (transport.AuthMethod, error) {
						return nil, nil
					},
				}
			},
			options: CloneOptions{
				Repo:    "https://github.com/nonexistent/repo.git",
				RefName: "refs/heads/main",
			},
			wantError:   true,
			expectError: "failed to clone repository",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cloner := &DefaultCloner{}
			authProvider := tc.setupAuthProvider()

			_, err := cloner.CloneRepository(memory.NewStorage(), memfs.New(), authProvider, tc.options)

			if tc.wantError {
				assert.Assert(t, err != nil)
				assert.ErrorContains(t, err, tc.expectError)
			} else {
				assert.NilError(t, err)
			}
		})
	}
}

// TestDefaultCloner_CloneRepository_Integration tests integration scenarios.
func TestDefaultCloner_CloneRepository_Integration(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tcs := map[string]struct {
		setupAuthProvider func() AuthProvider
		options           CloneOptions
		wantError         bool
	}{
		"SHA256HashRef": {
			setupAuthProvider: func() AuthProvider {
				return &MockAuthProvider{
					GetAuthMethodFunc: func() (transport.AuthMethod, error) {
						return nil, nil
					},
				}
			},
			options: CloneOptions{
				Repo:    "https://github.com/nonexistent/repo.git",
				RefName: "refs/heads/1234567890abcdef1234567890abcdef12345678", // Mock SHA format
			},
			wantError: true, // Expected to fail for nonexistent repo
		},
		"TagRef": {
			setupAuthProvider: func() AuthProvider {
				return &MockAuthProvider{
					GetAuthMethodFunc: func() (transport.AuthMethod, error) {
						return nil, nil
					},
				}
			},
			options: CloneOptions{
				Repo:    "https://github.com/nonexistent/repo.git",
				RefName: "refs/tags/v1.0.0",
			},
			wantError: true, // Expected to fail for nonexistent repo
		},
		"SparseCheckout": {
			setupAuthProvider: func() AuthProvider {
				return &MockAuthProvider{
					GetAuthMethodFunc: func() (transport.AuthMethod, error) {
						return nil, nil
					},
				}
			},
			options: CloneOptions{
				Repo:    "https://github.com/nonexistent/repo.git",
				RefName: "refs/heads/master",
				Path:    "some/path",
			},
			wantError: true, // Expected to fail for nonexistent repo
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cloner := &DefaultCloner{}
			authProvider := tc.setupAuthProvider()

			_, err := cloner.CloneRepository(memory.NewStorage(), memfs.New(), authProvider, tc.options)

			if tc.wantError {
				assert.Assert(t, err != nil)
			} else {
				assert.NilError(t, err)
			}
		})
	}
}

// TestHandleSHACheckout tests the handleSHACheckout function indirectly.
// Since it's a private function, we test it through the DefaultCloner.CloneRepository method.
func TestHandleSHACheckout(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		setupAuthProvider func() AuthProvider
		options           CloneOptions
		wantError         bool
		expectError       string
	}{
		"InvalidSHA": {
			setupAuthProvider: func() AuthProvider {
				return &MockAuthProvider{
					GetAuthMethodFunc: func() (transport.AuthMethod, error) {
						return nil, nil
					},
				}
			},
			options: CloneOptions{
				Repo:    "https://github.com/nonexistent/repo.git",
				RefName: "1234567890abcdef1234567890abcdef12345678", // Valid SHA format
			},
			wantError:   true,
			expectError: "failed to clone repository",
		},
		"SHAWithSparseCheckout": {
			setupAuthProvider: func() AuthProvider {
				return &MockAuthProvider{
					GetAuthMethodFunc: func() (transport.AuthMethod, error) {
						return nil, nil
					},
				}
			},
			options: CloneOptions{
				Repo:    "https://github.com/nonexistent/repo.git",
				RefName: "1234567890abcdef1234567890abcdef12345678",
				Path:    "some/path",
			},
			wantError:   true,
			expectError: "failed to clone repository",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cloner := &DefaultCloner{}
			authProvider := tc.setupAuthProvider()

			_, err := cloner.CloneRepository(memory.NewStorage(), memfs.New(), authProvider, tc.options)

			if tc.wantError {
				assert.Assert(t, err != nil)
				assert.ErrorContains(t, err, tc.expectError)
			} else {
				assert.NilError(t, err)
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
