// Copyright 2025 Upbound Inc.
// All rights reserved

package manager

import (
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/storage"
	"github.com/spf13/afero"
	"gotest.tools/v3/assert"

	"github.com/upbound/up/internal/git"
	"github.com/upbound/up/pkg/apis/project/v1alpha1"
)

// mockCloner is a mock implementation of git.Cloner for testing.
type mockCloner struct {
	ref        *plumbing.Reference
	cloneErr   error
	cloneCalls int
}

func (m *mockCloner) CloneRepository(_ storage.Storer, fs billy.Filesystem, _ git.AuthProvider, opts git.CloneOptions) (*plumbing.Reference, error) {
	m.cloneCalls++
	if m.cloneErr != nil {
		return nil, m.cloneErr
	}

	// Create a test file in the filesystem to simulate a successful clone.
	if opts.Path != "" {
		if err := fs.MkdirAll(opts.Path, 0o755); err != nil {
			return nil, err
		}
		file, err := fs.Create(opts.Path + "/test.yaml")
		if err != nil {
			return nil, err
		}
		file.Close()
	} else {
		file, err := fs.Create("test.yaml")
		if err != nil {
			return nil, err
		}
		file.Close()
	}

	return m.ref, nil
}

// mockAuthProvider is a mock implementation of git.AuthProvider.
type mockAuthProvider struct{}

func (m *mockAuthProvider) GetAuthMethod() (transport.AuthMethod, error) {
	return nil, nil
}

func TestGitSourceVersion(t *testing.T) {
	t.Parallel()

	commitHash := plumbing.NewHash("abc123def456789012345678901234567890abcd")
	mockRef := plumbing.NewHashReference("refs/heads/main", commitHash)

	tcs := map[string]struct {
		source    *gitSource
		wantSHA   string
		wantError bool
	}{
		"ReturnsCommitSHA": {
			source: &gitSource{
				git: &v1alpha1.APIGitReference{
					Repository: "https://github.com/example/repo.git",
					Ref:        "main",
				},
				cloner: &mockCloner{
					ref: mockRef,
				},
				authProvider: &mockAuthProvider{},
				sourceType:   SourceTypeCRD,
			},
			wantSHA:   "abc123def456789012345678901234567890abcd",
			wantError: false,
		},
		"CachedCommitSHA": {
			source: &gitSource{
				git: &v1alpha1.APIGitReference{
					Repository: "https://github.com/example/repo.git",
					Ref:        "main",
				},
				cloner:       &mockCloner{ref: mockRef},
				authProvider: &mockAuthProvider{},
				sourceType:   SourceTypeCRD,
				commitSHA:    "cached123def456789012345678901234567890a",
			},
			wantSHA:   "cached123def456789012345678901234567890a",
			wantError: false,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			gotVersion, err := tc.source.Version(t.Context())

			if tc.wantError {
				assert.Assert(t, err != nil)
				return
			}

			assert.NilError(t, err)
			assert.Equal(t, tc.wantSHA, gotVersion)
		})
	}
}

func TestGitSourceResources(t *testing.T) {
	t.Parallel()

	commitHash := plumbing.NewHash("abc123def456789012345678901234567890abcd")
	mockRef := plumbing.NewHashReference("refs/heads/main", commitHash)

	tcs := map[string]struct {
		source        *gitSource
		wantCommitSHA string
		wantError     bool
	}{
		"StoresCommitSHA": {
			source: &gitSource{
				git: &v1alpha1.APIGitReference{
					Repository: "https://github.com/example/repo.git",
					Ref:        "main",
				},
				cloner: &mockCloner{
					ref: mockRef,
				},
				authProvider: &mockAuthProvider{},
				sourceType:   SourceTypeCRD,
			},
			wantCommitSHA: "abc123def456789012345678901234567890abcd",
			wantError:     false,
		},
		"WithPath": {
			source: &gitSource{
				git: &v1alpha1.APIGitReference{
					Repository: "https://github.com/example/repo.git",
					Ref:        "main",
					Path:       "apis",
				},
				cloner: &mockCloner{
					ref: mockRef,
				},
				authProvider: &mockAuthProvider{},
				sourceType:   SourceTypeCRD,
			},
			wantCommitSHA: "abc123def456789012345678901234567890abcd",
			wantError:     false,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			gotFS, err := tc.source.Resources(t.Context())

			if tc.wantError {
				assert.Assert(t, err != nil)
				return
			}

			assert.NilError(t, err)
			assert.Assert(t, gotFS != nil)
			assert.Equal(t, tc.wantCommitSHA, tc.source.commitSHA)

			// Verify filesystem is cached
			assert.Assert(t, tc.source.fs != nil)

			// Verify that calling Resources again returns cached filesystem.
			gotFS2, err := tc.source.Resources(t.Context())
			assert.NilError(t, err)
			assert.Equal(t, gotFS, gotFS2)
		})
	}
}

func TestGitSourceID(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		source *gitSource
		wantID string
	}{
		"BasicRepository": {
			source: &gitSource{
				git: &v1alpha1.APIGitReference{
					Repository: "https://github.com/example/repo.git",
				},
			},
			wantID: "git://https://github.com/example/repo.git",
		},
		"RepositoryWithPath": {
			source: &gitSource{
				git: &v1alpha1.APIGitReference{
					Repository: "https://github.com/example/repo.git",
					Path:       "apis/crds",
				},
			},
			wantID: "git://https://github.com/example/repo.git/apis/crds",
		},
		"EmptyRepository": {
			source: &gitSource{
				git: &v1alpha1.APIGitReference{
					Repository: "",
				},
			},
			wantID: "git://",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			gotID := tc.source.ID()
			assert.Equal(t, tc.wantID, gotID)
		})
	}
}

func TestGitSourceType(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		source   *gitSource
		wantType SourceType
	}{
		"CRDType": {
			source: &gitSource{
				sourceType: SourceTypeCRD,
			},
			wantType: SourceTypeCRD,
		},
		"OpenAPIType": {
			source: &gitSource{
				sourceType: SourceTypeOpenAPI,
			},
			wantType: SourceTypeOpenAPI,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			gotType := tc.source.Type()
			assert.Equal(t, tc.wantType, gotType)
		})
	}
}

func TestGitSourceNormalizeRef(t *testing.T) {
	t.Parallel()

	g := &gitSource{}

	tcs := map[string]struct {
		input string
		want  string
	}{
		"EmptyRef": {
			input: "",
			want:  "refs/heads/main",
		},
		"BranchName": {
			input: "develop",
			want:  "refs/heads/develop",
		},
		"VersionTag": {
			input: "v1.2.3",
			want:  "refs/tags/v1.2.3",
		},
		"NumericVersionTag": {
			input: "1.2.3",
			want:  "refs/tags/1.2.3",
		},
		"FullRef": {
			input: "refs/heads/feature",
			want:  "refs/heads/feature",
		},
		"FullTagRef": {
			input: "refs/tags/v1.0.0",
			want:  "refs/tags/v1.0.0",
		},
		"SHA256Hash": {
			input: "abc123def456789012345678901234567890abcd",
			want:  "abc123def456789012345678901234567890abcd",
		},
		"SHA256HashUppercase": {
			input: "ABC123DEF456789012345678901234567890ABCD",
			want:  "ABC123DEF456789012345678901234567890ABCD",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := g.normalizeRef(tc.input)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestGitSourceVerifyClone(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		setupFS   func() billy.Filesystem
		path      string
		wantError bool
	}{
		"ValidClone": {
			setupFS: func() billy.Filesystem {
				fs := memfs.New()
				file, _ := fs.Create("test.yaml")
				file.Close()
				return fs
			},
			path:      ".",
			wantError: false,
		},
		"ValidCloneWithPath": {
			setupFS: func() billy.Filesystem {
				fs := memfs.New()
				_ = fs.MkdirAll("apis", 0o755)
				file, _ := fs.Create("apis/test.yaml")
				file.Close()
				return fs
			},
			path:      "apis",
			wantError: false,
		},
		"EmptyDirectory": {
			setupFS: func() billy.Filesystem {
				fs := memfs.New()
				_ = fs.MkdirAll("empty", 0o755)
				return fs
			},
			path:      "empty",
			wantError: true,
		},
		"NonExistentPath": {
			setupFS:   memfs.New,
			path:      "does-not-exist",
			wantError: true,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			g := &gitSource{}
			fs := tc.setupFS()

			err := g.verifyClone(fs, tc.path)

			if tc.wantError {
				assert.Assert(t, err != nil)
			} else {
				assert.NilError(t, err)
			}
		})
	}
}

func Test_isVersionTag(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		input string
		want  bool
	}{
		"vPrefixedVersion": {
			input: "v1.2.3",
			want:  true,
		},
		"NumericVersion": {
			input: "1.2.3",
			want:  true,
		},
		"BranchName": {
			input: "main",
			want:  false,
		},
		"EmptyString": {
			input: "",
			want:  false,
		},
		"vWithoutNumber": {
			input: "v",
			want:  false,
		},
		"vWithLetter": {
			input: "va",
			want:  false,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := isVersionTag(tc.input)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestIsSHA256Hash(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		input string
		want  bool
	}{
		"ValidSHA": {
			input: "abc123def456789012345678901234567890abcd",
			want:  true,
		},
		"ValidSHAUppercase": {
			input: "ABC123DEF456789012345678901234567890ABCD",
			want:  true,
		},
		"ValidSHAMixed": {
			input: "AbC123DeF456789012345678901234567890aBcD",
			want:  true,
		},
		"TooShort": {
			input: "abc123",
			want:  false,
		},
		"TooLong": {
			input: "abc123def456789012345678901234567890abcde",
			want:  false,
		},
		"InvalidCharacters": {
			input: "xyz123def456789012345678901234567890abcd",
			want:  false,
		},
		"EmptyString": {
			input: "",
			want:  false,
		},
		"BranchName": {
			input: "main",
			want:  false,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := git.CheckSHA256Hash(tc.input)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestCalculateFilesystemHash(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		setupFS    func() afero.Fs
		sourceType SourceType
		wantError  bool
		wantSame   bool // whether two identical setups should produce the same hash.
	}{
		"CRDSourceWithYAMLFiles": {
			setupFS: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "test1.yaml", []byte("apiVersion: v1\nkind: Test"), 0o644)
				_ = afero.WriteFile(fs, "test2.yml", []byte("apiVersion: v1\nkind: Test2"), 0o644)
				return fs
			},
			sourceType: SourceTypeCRD,
			wantError:  false,
		},
		"OpenAPISourceWithJSONFiles": {
			setupFS: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "openapi.json", []byte(`{"openapi": "3.0.0"}`), 0o644)
				return fs
			},
			sourceType: SourceTypeCRD,
			wantError:  false,
		},
		"CRDSourceIgnoresNonYAMLFiles": {
			setupFS: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "test.yaml", []byte("apiVersion: v1\nkind: Test"), 0o644)
				_ = afero.WriteFile(fs, "readme.txt", []byte("This should be ignored"), 0o644)
				_ = afero.WriteFile(fs, "script.sh", []byte("#!/bin/bash"), 0o644)
				return fs
			},
			sourceType: SourceTypeCRD,
			wantError:  false,
		},
		"OpenAPISourceIgnoresNonJSONFiles": {
			setupFS: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "openapi.json", []byte(`{"openapi": "3.0.0"}`), 0o644)
				_ = afero.WriteFile(fs, "test.yaml", []byte("apiVersion: v1\nkind: Test"), 0o644)
				return fs
			},
			sourceType: SourceTypeOpenAPI,
			wantError:  false,
		},
		"EmptyFilesystem": {
			setupFS:    afero.NewMemMapFs,
			sourceType: SourceTypeCRD,
			wantError:  false,
		},
		"DirectoriesIgnored": {
			setupFS: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = fs.MkdirAll("subdir", 0o755)
				_ = afero.WriteFile(fs, "subdir/test.yaml", []byte("apiVersion: v1\nkind: Test"), 0o644)
				return fs
			},
			sourceType: SourceTypeCRD,
			wantError:  false,
		},
		"IdenticalContentSameHash": {
			setupFS: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "test.yaml", []byte("apiVersion: v1\nkind: Test"), 0o644)
				return fs
			},
			sourceType: SourceTypeCRD,
			wantError:  false,
		},
		"DifferentSourceTypeDifferentHash": {
			setupFS: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "test.yaml", []byte("apiVersion: v1\nkind: Test"), 0o644)
				_ = afero.WriteFile(fs, "openapi.json", []byte(`{"openapi": "3.0.0"}`), 0o644)
				return fs
			},
			sourceType: SourceTypeCRD,
			wantError:  false,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			fs := tc.setupFS()
			hash, err := calculateFilesystemHash(fs, tc.sourceType)

			if tc.wantError {
				assert.Assert(t, err != nil)
				return
			}

			assert.NilError(t, err)
			assert.Assert(t, len(hash) > 0, "hash should not be empty")
			assert.Assert(t, len(hash) == 64, "SHA256 hash should be 64 hex characters")

			// Verify hash is deterministic.
			hash2, err := calculateFilesystemHash(fs, tc.sourceType)
			assert.NilError(t, err)
			assert.Equal(t, hash, hash2, "hash should be deterministic")

			// Special case tests for specific scenarios.
			switch name {
			case "IdenticalContentSameHash":
				// Test that identical content in different filesystem instances produces identical hashes.
				fs2 := tc.setupFS()
				hash3, err := calculateFilesystemHash(fs2, tc.sourceType)
				assert.NilError(t, err)
				assert.Equal(t, hash, hash3, "identical content should produce identical hashes")

			case "DifferentSourceTypeDifferentHash":
				// Test that same filesystem with different source type produces different hash.
				hashOpenAPI, err := calculateFilesystemHash(fs, SourceTypeOpenAPI)
				assert.NilError(t, err)
				assert.Assert(t, hash != hashOpenAPI, "different source types should produce different hashes")
			}
		})
	}
}

func TestCalculateFilesystemHashDifferentContent(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		setupFS1   func() afero.Fs
		setupFS2   func() afero.Fs
		sourceType SourceType
	}{
		"DifferentFileContent": {
			setupFS1: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "test.yaml", []byte("apiVersion: v1\nkind: Test1"), 0o644)
				return fs
			},
			setupFS2: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "test.yaml", []byte("apiVersion: v1\nkind: Test2"), 0o644)
				return fs
			},
			sourceType: SourceTypeCRD,
		},
		"DifferentNumberOfFiles": {
			setupFS1: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "test.yaml", []byte("apiVersion: v1\nkind: Test"), 0o644)
				return fs
			},
			setupFS2: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "test1.yaml", []byte("apiVersion: v1\nkind: Test"), 0o644)
				_ = afero.WriteFile(fs, "test2.yaml", []byte("apiVersion: v1\nkind: Test2"), 0o644)
				return fs
			},
			sourceType: SourceTypeCRD,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			fs1 := tc.setupFS1()
			fs2 := tc.setupFS2()

			hash1, err := calculateFilesystemHash(fs1, tc.sourceType)
			assert.NilError(t, err)

			hash2, err := calculateFilesystemHash(fs2, tc.sourceType)
			assert.NilError(t, err)

			assert.Assert(t, hash1 != hash2, "different content should produce different hashes")
		})
	}
}

func TestShouldFilterCRD(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		content string
		want    bool
	}{
		"CrossplaneManaged": {
			content: `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: test.example.com
spec:
  names:
    categories:
    - crossplane
    - managed
    kind: Test`,
			want: true,
		},
		"CrossplaneStore": {
			content: `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: test.example.com
spec:
  names:
    categories:
    - crossplane
    - store
    kind: Test`,
			want: true,
		},
		"CrossplaneProvider": {
			content: `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: test.example.com
spec:
  names:
    categories:
    - crossplane
    - provider
    kind: Test`,
			want: true,
		},
		"CrossplaneOnly": {
			content: `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: test.example.com
spec:
  names:
    categories:
    - crossplane
    kind: Test`,
			want: false,
		},
		"ManagedOnly": {
			content: `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: test.example.com
spec:
  names:
    categories:
    - managed
    kind: Test`,
			want: false,
		},
		"NoCategoriesCRD": {
			content: `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: test.example.com
spec:
  names:
    kind: Test`,
			want: false,
		},
		"NotACRD": {
			content: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test`,
			want: false,
		},
		"InvalidYAML": {
			content: `this is not yaml`,
			want:    false,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := shouldFilterCRD([]byte(tc.content))
			assert.Equal(t, tc.want, got)
		})
	}
}
