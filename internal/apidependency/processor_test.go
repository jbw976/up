// Copyright 2025 Upbound Inc.
// All rights reserved

package apidependency

import (
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/storage"
	"github.com/spf13/afero"
	"gotest.tools/v3/assert"

	"github.com/upbound/up/internal/git"
	"github.com/upbound/up/internal/schemas/manager"
	"github.com/upbound/up/pkg/apis/project/v2alpha1"
)

// mockCloner implements git.Cloner for testing.
type mockCloner struct {
	ref      *plumbing.Reference
	cloneErr error
}

func (m *mockCloner) CloneRepository(_ storage.Storer, fs billy.Filesystem, _ git.AuthProvider, opts git.CloneOptions) (*plumbing.Reference, error) {
	if m.cloneErr != nil {
		return nil, m.cloneErr
	}

	// Create a test file to simulate successful clone.
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

// mockAuthProvider implements git.AuthProvider for testing.
type mockAuthProvider struct{}

func (m *mockAuthProvider) GetAuthMethod() (transport.AuthMethod, error) {
	return nil, nil
}

// mockCache implements Cache for testing.
type mockCache struct {
	storage  map[string]afero.Fs
	getErr   error
	storeErr error
}

func newMockCache() *mockCache {
	return &mockCache{
		storage: make(map[string]afero.Fs),
	}
}

func (m *mockCache) Get(dep v2alpha1.APIDependencies) (afero.Fs, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	key := m.calculateKey(dep)
	if fs, exists := m.storage[key]; exists {
		return fs, nil
	}
	return nil, afero.ErrFileNotFound
}

func (m *mockCache) Store(dep v2alpha1.APIDependencies, fs afero.Fs) error {
	if m.storeErr != nil {
		return m.storeErr
	}
	key := m.calculateKey(dep)
	m.storage[key] = fs
	return nil
}

func (m *mockCache) calculateKey(dep v2alpha1.APIDependencies) string {
	// Simple key calculation for testing.
	if dep.Git != nil {
		return "git:" + dep.Git.Repository
	}
	if dep.HTTP != nil {
		return "http:" + dep.HTTP.URL
	}
	if dep.K8s != nil {
		return "k8s:" + dep.K8s.Version
	}
	return "unknown"
}

func TestProcessorProcess(t *testing.T) {
	t.Parallel()

	commitHash := plumbing.NewHash("abc123def456789012345678901234567890abcd")
	mockRef := plumbing.NewHashReference("refs/heads/main", commitHash)

	tcs := map[string]struct {
		dep       v2alpha1.APIDependencies
		processor *Processor
		wantErr   bool
		wantType  manager.SourceType
	}{
		"GitDependencyCRD": {
			dep: v2alpha1.APIDependencies{
				Type: v2alpha1.APIDependencyTypeCRD,
				Git: &v2alpha1.APIGitReference{
					Repository: "https://github.com/example/repo.git",
					Ref:        "main",
					Path:       "apis",
				},
			},
			processor: NewProcessor(
				&mockCloner{ref: mockRef},
				&mockAuthProvider{},
				newMockCache(),
			),
			wantErr:  false,
			wantType: manager.SourceTypeCRD,
		},
		"K8sDependency": {
			dep: v2alpha1.APIDependencies{
				Type: v2alpha1.APIDependencyTypeK8s,
				K8s: &v2alpha1.APIK8sReference{
					Version: "v1.28.0",
				},
			},
			processor: NewProcessor(
				&mockCloner{ref: mockRef},
				&mockAuthProvider{},
				newMockCache(),
			),
			wantErr:  false,
			wantType: manager.SourceTypeOpenAPI,
		},
		"K8sDependencyMissingConfig": {
			dep: v2alpha1.APIDependencies{
				Type: v2alpha1.APIDependencyTypeK8s,
			},
			processor: NewProcessor(
				&mockCloner{ref: mockRef},
				&mockAuthProvider{},
				newMockCache(),
			),
			wantErr: true,
		},
		"NoSourceConfigured": {
			dep: v2alpha1.APIDependencies{
				Type: v2alpha1.APIDependencyTypeCRD,
			},
			processor: NewProcessor(
				&mockCloner{ref: mockRef},
				&mockAuthProvider{},
				newMockCache(),
			),
			wantErr: true,
		},
		"ProcessorWithoutCache": {
			dep: v2alpha1.APIDependencies{
				Type: v2alpha1.APIDependencyTypeCRD,
				Git: &v2alpha1.APIGitReference{
					Repository: "https://github.com/example/repo.git",
					Ref:        "main",
				},
			},
			processor: NewProcessor(
				&mockCloner{ref: mockRef},
				&mockAuthProvider{},
				nil, // no cache
			),
			wantErr:  false,
			wantType: manager.SourceTypeCRD,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			source, err := tc.processor.Process(tc.dep)

			if tc.wantErr {
				assert.Assert(t, err != nil)
				return
			}

			assert.NilError(t, err)
			assert.Assert(t, source != nil)
			assert.Equal(t, tc.wantType, source.Type())

			// Verify we can get resources.
			_, err = source.Resources(t.Context())
			assert.NilError(t, err)

			// Verify we can get version.
			_, err = source.Version(t.Context())
			assert.NilError(t, err)

			// Verify ID is not empty.
			id := source.ID()
			assert.Assert(t, len(id) > 0)
		})
	}
}

func TestProcessorK8sConversion(t *testing.T) {
	t.Parallel()

	commitHash := plumbing.NewHash("abc123def456789012345678901234567890abcd")
	mockRef := plumbing.NewHashReference("refs/heads/main", commitHash)

	processor := NewProcessor(
		&mockCloner{ref: mockRef},
		&mockAuthProvider{},
		newMockCache(),
	)

	dep := v2alpha1.APIDependencies{
		Type: v2alpha1.APIDependencyTypeK8s,
		K8s: &v2alpha1.APIK8sReference{
			Version: "v1.28.0",
		},
	}

	source, err := processor.Process(dep)
	assert.NilError(t, err)
	assert.Assert(t, source != nil)

	// Verify the source type is OpenAPI for K8s dependencies.
	assert.Equal(t, manager.SourceTypeOpenAPI, source.Type())

	// Verify the source ID contains the expected repository.
	id := source.ID()
	assert.Assert(t, id == "git://https://github.com/kubernetes/kubernetes/api/openapi-spec")
}

func TestProcessorCacheIntegration(t *testing.T) {
	t.Parallel()

	commitHash := plumbing.NewHash("abc123def456789012345678901234567890abcd")
	mockRef := plumbing.NewHashReference("refs/heads/main", commitHash)

	tcs := map[string]struct {
		dep       v2alpha1.APIDependencies
		processor *Processor
	}{
		"CacheHitAndMiss": {
			dep: v2alpha1.APIDependencies{
				Type: v2alpha1.APIDependencyTypeCRD,
				Git: &v2alpha1.APIGitReference{
					Repository: "https://github.com/example/repo.git",
					Ref:        "main",
				},
			},
			processor: NewProcessor(
				&mockCloner{ref: mockRef},
				&mockAuthProvider{},
				newMockCache(),
			),
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// First call should miss cache and populate it.
			source1, err := tc.processor.Process(tc.dep)
			assert.NilError(t, err)

			// Get resources to trigger caching.
			fs1, err := source1.Resources(t.Context())
			assert.NilError(t, err)
			assert.Assert(t, fs1 != nil)

			// Second call should hit cache.
			source2, err := tc.processor.Process(tc.dep)
			assert.NilError(t, err)

			// Both sources should have the same ID.
			assert.Equal(t, source1.ID(), source2.ID())
		})
	}
}

func TestNewProcessor(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		cloner       git.Cloner
		authProvider git.AuthProvider
		cache        Cache
	}{
		"ValidProcessor": {
			cloner:       &mockCloner{},
			authProvider: &mockAuthProvider{},
			cache:        newMockCache(),
		},
		"ProcessorWithNilCache": {
			cloner:       &mockCloner{},
			authProvider: &mockAuthProvider{},
			cache:        nil,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			processor := NewProcessor(tc.cloner, tc.authProvider, tc.cache)

			assert.Assert(t, processor != nil)
			assert.Equal(t, tc.cloner, processor.cloner)
			assert.Equal(t, tc.authProvider, processor.authProvider)
			assert.Equal(t, tc.cache, processor.cache)
		})
	}
}
