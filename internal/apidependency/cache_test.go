// Copyright 2025 Upbound Inc.
// All rights reserved

package apidependency

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"gotest.tools/v3/assert"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	"github.com/upbound/up/pkg/apis/project/v2alpha1"
)

func TestNewLocalCache(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		root    string
		opts    []LocalCacheOption
		wantErr bool
	}{
		"ValidCache": {
			root:    "/tmp/test-cache",
			opts:    []LocalCacheOption{},
			wantErr: false,
		},
		"CacheWithLogger": {
			root: "/tmp/test-cache-logger",
			opts: []LocalCacheOption{
				WithLogger(logging.NewNopLogger()),
			},
			wantErr: false,
		},
		"CacheWithCustomFS": {
			root: "/tmp/test-cache-fs",
			opts: []LocalCacheOption{
				WithFS(afero.NewMemMapFs()),
			},
			wantErr: false,
		},
		"CacheWithAllOptions": {
			root: "/tmp/test-cache-all",
			opts: []LocalCacheOption{
				WithLogger(logging.NewNopLogger()),
				WithFS(afero.NewMemMapFs()),
			},
			wantErr: false,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cache, err := NewLocalCache(tc.root, tc.opts...)

			if tc.wantErr {
				assert.Assert(t, err != nil)
				return
			}

			assert.NilError(t, err)
			assert.Assert(t, cache != nil)
			assert.Equal(t, filepath.Clean(tc.root), cache.root)
			assert.Assert(t, cache.log != nil)
			assert.Assert(t, cache.fs != nil)
		})
	}
}

func TestLocalCacheGetNotFound(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		dep v2alpha1.APIDependencies
	}{
		"GitDependency": {
			dep: v2alpha1.APIDependencies{
				Type: v2alpha1.APIDependencyTypeCRD,
				Git: &v2alpha1.APIGitReference{
					Repository: "https://github.com/example/repo.git",
					Ref:        "main",
				},
			},
		},
		"HTTPDependency": {
			dep: v2alpha1.APIDependencies{
				Type: v2alpha1.APIDependencyTypeCRD,
				HTTP: &v2alpha1.APIHTTPReference{
					URL: "https://example.com/crd.yaml",
				},
			},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cache, err := NewLocalCache("/tmp/test-cache", WithFS(afero.NewMemMapFs()))
			assert.NilError(t, err)

			fs, err := cache.Get(tc.dep)
			assert.Equal(t, os.ErrNotExist, err)
			assert.Assert(t, fs == nil)
		})
	}
}

func TestLocalCacheStoreAndGet(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		dep     v2alpha1.APIDependencies
		setupFS func() afero.Fs
	}{
		"GitDependencyWithFiles": {
			dep: v2alpha1.APIDependencies{
				Type: v2alpha1.APIDependencyTypeCRD,
				Git: &v2alpha1.APIGitReference{
					Repository: "https://github.com/example/repo.git",
					Ref:        "main",
					Path:       "apis",
				},
			},
			setupFS: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "test.yaml", []byte("apiVersion: v1\nkind: Test"), 0o644)
				_ = afero.WriteFile(fs, "subdir/test2.yaml", []byte("apiVersion: v1\nkind: Test2"), 0o644)
				return fs
			},
		},
		"HTTPDependencyWithFile": {
			dep: v2alpha1.APIDependencies{
				Type: v2alpha1.APIDependencyTypeCRD,
				HTTP: &v2alpha1.APIHTTPReference{
					URL: "https://example.com/crd.yaml",
				},
			},
			setupFS: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "crd.yaml", []byte("apiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition"), 0o644)
				return fs
			},
		},
		"EmptyFilesystem": {
			dep: v2alpha1.APIDependencies{
				Type: v2alpha1.APIDependencyTypeCRD,
				Git: &v2alpha1.APIGitReference{
					Repository: "https://github.com/empty/repo.git",
					Ref:        "main",
				},
			},
			setupFS: afero.NewMemMapFs,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cache, err := NewLocalCache("/tmp/test-cache", WithFS(afero.NewMemMapFs()))
			assert.NilError(t, err)

			sourceFS := tc.setupFS()

			// Store the filesystem
			err = cache.Store(tc.dep, sourceFS)
			assert.NilError(t, err)

			// Retrieve the filesystem
			cachedFS, err := cache.Get(tc.dep)
			assert.NilError(t, err)
			assert.Assert(t, cachedFS != nil)

			// Verify files are present in cached filesystem
			sourceFiles := getFilesFromFS(sourceFS)
			cachedFiles := getFilesFromFS(cachedFS)
			assert.DeepEqual(t, sourceFiles, cachedFiles)
		})
	}
}

func TestLocalCacheStoreOverwrite(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		dep      v2alpha1.APIDependencies
		setupFS1 func() afero.Fs
		setupFS2 func() afero.Fs
	}{
		"OverwriteWithDifferentContent": {
			dep: v2alpha1.APIDependencies{
				Type: v2alpha1.APIDependencyTypeCRD,
				Git: &v2alpha1.APIGitReference{
					Repository: "https://github.com/example/repo.git",
					Ref:        "main",
				},
			},
			setupFS1: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "test.yaml", []byte("content1"), 0o644)
				return fs
			},
			setupFS2: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "test.yaml", []byte("content2"), 0o644)
				return fs
			},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cache, err := NewLocalCache("/tmp/test-cache", WithFS(afero.NewMemMapFs()))
			assert.NilError(t, err)

			// Store first filesystem
			fs1 := tc.setupFS1()
			err = cache.Store(tc.dep, fs1)
			assert.NilError(t, err)

			// Store second filesystem (overwrite)
			fs2 := tc.setupFS2()
			err = cache.Store(tc.dep, fs2)
			assert.NilError(t, err)

			// Retrieve and verify it's the second content
			cachedFS, err := cache.Get(tc.dep)
			assert.NilError(t, err)

			content, err := afero.ReadFile(cachedFS, "test.yaml")
			assert.NilError(t, err)
			assert.Equal(t, "content2", string(content))
		})
	}
}

func TestLocalCacheCalculateKey(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		dep1     v2alpha1.APIDependencies
		dep2     v2alpha1.APIDependencies
		wantSame bool
	}{
		"SameGitDependency": {
			dep1: v2alpha1.APIDependencies{
				Type: v2alpha1.APIDependencyTypeCRD,
				Git: &v2alpha1.APIGitReference{
					Repository: "https://github.com/example/repo.git",
					Ref:        "main",
					Path:       "apis",
				},
			},
			dep2: v2alpha1.APIDependencies{
				Type: v2alpha1.APIDependencyTypeCRD,
				Git: &v2alpha1.APIGitReference{
					Repository: "https://github.com/example/repo.git",
					Ref:        "main",
					Path:       "apis",
				},
			},
			wantSame: true,
		},
		"DifferentGitRef": {
			dep1: v2alpha1.APIDependencies{
				Type: v2alpha1.APIDependencyTypeCRD,
				Git: &v2alpha1.APIGitReference{
					Repository: "https://github.com/example/repo.git",
					Ref:        "main",
				},
			},
			dep2: v2alpha1.APIDependencies{
				Type: v2alpha1.APIDependencyTypeCRD,
				Git: &v2alpha1.APIGitReference{
					Repository: "https://github.com/example/repo.git",
					Ref:        "develop",
				},
			},
			wantSame: false,
		},
		"DifferentType": {
			dep1: v2alpha1.APIDependencies{
				Type: v2alpha1.APIDependencyTypeCRD,
				Git: &v2alpha1.APIGitReference{
					Repository: "https://github.com/example/repo.git",
					Ref:        "main",
				},
			},
			dep2: v2alpha1.APIDependencies{
				Type: v2alpha1.APIDependencyTypeK8s,
				Git: &v2alpha1.APIGitReference{
					Repository: "https://github.com/example/repo.git",
					Ref:        "main",
				},
			},
			wantSame: false,
		},
		"GitVsHTTP": {
			dep1: v2alpha1.APIDependencies{
				Type: v2alpha1.APIDependencyTypeCRD,
				Git: &v2alpha1.APIGitReference{
					Repository: "https://github.com/example/repo.git",
					Ref:        "main",
				},
			},
			dep2: v2alpha1.APIDependencies{
				Type: v2alpha1.APIDependencyTypeCRD,
				HTTP: &v2alpha1.APIHTTPReference{
					URL: "https://example.com/crd.yaml",
				},
			},
			wantSame: false,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cache, err := NewLocalCache("/tmp/test-cache", WithFS(afero.NewMemMapFs()))
			assert.NilError(t, err)

			key1 := cache.calculateKey(tc.dep1)
			key2 := cache.calculateKey(tc.dep2)

			if tc.wantSame {
				assert.Equal(t, key1, key2)
			} else {
				assert.Assert(t, key1 != key2)
			}

			// Verify keys are valid hex strings of expected length
			assert.Equal(t, 16, len(key1))
			assert.Equal(t, 16, len(key2))
		})
	}
}

func TestLocalCacheEntryPath(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		root string
		key  string
		want string
	}{
		"SimpleKey": {
			root: "/tmp/cache",
			key:  "abc123",
			want: "/tmp/cache/abc123",
		},
		"KeyWithSpecialChars": {
			root: "/tmp/cache",
			key:  "abc123def456",
			want: "/tmp/cache/abc123def456",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cache, err := NewLocalCache(tc.root, WithFS(afero.NewMemMapFs()))
			assert.NilError(t, err)

			got := cache.entryPath(tc.key)
			assert.Equal(t, tc.want, got)
		})
	}
}

// getFilesFromFS returns a map of file paths to their content for comparison.
func getFilesFromFS(fs afero.Fs) map[string]string {
	files := make(map[string]string)
	_ = afero.Walk(fs, ".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		content, _ := afero.ReadFile(fs, path)
		files[path] = string(content)
		return nil
	})
	return files
}
