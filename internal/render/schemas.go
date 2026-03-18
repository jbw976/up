// Copyright 2025 Upbound Inc.
// All rights reserved

package render

import (
	"context"
	"encoding/json"
	"path/filepath"

	"github.com/spf13/afero"
	apiextensionsv1client "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/kube-openapi/pkg/spec3"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	v1 "github.com/crossplane/crossplane/v2/apis/apiextensions/v1"
	pkgmetav1 "github.com/crossplane/crossplane/v2/apis/pkg/meta/v1"
	"github.com/crossplane/crossplane/v2/xcrd"

	"github.com/upbound/up/internal/apidependency"
	icrd "github.com/upbound/up/internal/crd"
	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/pkg/apis/project/v2alpha1"
)

// SchemaOptions contains all the options needed to load schemas from various sources.
type SchemaOptions struct {
	Project           *v2alpha1.Project
	XRD               *v1.CompositeResourceDefinition
	DependencyManager *project.DependencyManager
}

// LoadAllSchemas loads all available schemas from
// API dependencies , XRD, and CRDs from project cache.
func LoadAllSchemas(ctx context.Context, opts SchemaOptions) []spec3.OpenAPI {
	var schemas []spec3.OpenAPI

	// Load schemas from cached API dependencies (e.g. Kubernetes OpenAPI schemas)
	if opts.DependencyManager != nil && opts.Project != nil && len(opts.Project.Spec.APIDependencies) > 0 {
		cachedSchemas := loadCachedSchemas(
			opts.DependencyManager.APIDependencyCache(),
			opts.Project.Spec.APIDependencies,
		)
		schemas = append(schemas, cachedSchemas...)
	}

	// Load CRD schemas from XRD
	if opts.XRD != nil {
		// Convert XRD to CRD
		crd, err := xcrd.ForCompositeResource(opts.XRD)
		if err == nil {
			xrdSchemas := crdToSchemas(crd)
			schemas = append(schemas, xrdSchemas...)
		}
	}

	// Load CRD schemas from crossplane dependencies
	if opts.DependencyManager != nil && opts.Project != nil && len(opts.Project.Spec.DependsOn) > 0 {
		depSchemas := loadDependencyCRDSchemas(ctx, opts.DependencyManager, opts.Project.Spec.DependsOn)
		schemas = append(schemas, depSchemas...)
	}

	return schemas
}

// loadDependencyCRDSchemas loads CRD schemas from dependsOn packages.
func loadDependencyCRDSchemas(ctx context.Context, dm *project.DependencyManager, deps []pkgmetav1.Dependency) []spec3.OpenAPI {
	var schemas []spec3.OpenAPI

	for _, dep := range deps {
		pkg, err := dm.GetParsedPackage(ctx, dep)
		if err != nil {
			// Skip dependencies that aren't cached yet
			continue
		}

		// Extract CRDs from the package
		for _, obj := range pkg.Objs {
			crd, ok := obj.(*apiextensionsv1client.CustomResourceDefinition)
			if !ok {
				continue
			}

			// Convert CRD to schemas
			crdSchemas := crdToSchemas(crd)
			schemas = append(schemas, crdSchemas...)
		}
	}

	return schemas
}

// loadCachedSchemas loads OpenAPI v3 schemas from the cache for the given
// API dependencies and returns them as spec3.OpenAPI.
func loadCachedSchemas(cache apidependency.Cache, deps []v2alpha1.APIDependencies) []spec3.OpenAPI {
	if cache == nil || len(deps) == 0 {
		return nil
	}

	var schemas []spec3.OpenAPI

	for _, dep := range deps {
		// Only look in K8s type dependencies that have OpenAPI schemas
		if dep.Type != v2alpha1.APIDependencyTypeK8s {
			continue
		}

		// Convert K8s config to Git config if needed (same as processor does)
		cacheLookupDep := dep
		if dep.K8s != nil && dep.Git == nil {
			cacheLookupDep = v2alpha1.APIDependencies{
				Type: v2alpha1.APIDependencyTypeK8s,
				Git: &v2alpha1.APIGitReference{
					Repository: "https://github.com/kubernetes/kubernetes",
					Ref:        dep.K8s.Version,
					Path:       "api/openapi-spec",
				},
			}
		}

		// Get the cached filesystem for this dependency
		fs, err := cache.Get(cacheLookupDep)
		if err != nil {
			continue
		}

		// Load all OpenAPI schema files from the cache
		depSchemas := loadSchemasFromFS(fs)
		schemas = append(schemas, depSchemas...)
	}

	return schemas
}

// crdToSchemas converts a CRD to OpenAPI v3 schema documents.
// This works for both regular CRDs and CRDs derived from XRDs.
// Returns one spec3.OpenAPI per CRD version.
func crdToSchemas(crd *apiextensionsv1client.CustomResourceDefinition) []spec3.OpenAPI {
	oapis, err := icrd.ToOpenAPI(crd)
	if err != nil {
		return nil
	}

	// Convert map[string]*spec3.OpenAPI to []spec3.OpenAPI
	schemas := make([]spec3.OpenAPI, 0, len(oapis))
	for _, oapi := range oapis {
		if oapi != nil {
			schemas = append(schemas, *oapi)
		}
	}

	return schemas
}

// loadSchemasFromFS loads all OpenAPI v3 schema files from the filesystem.
// It searches for JSON files in the typical Kubernetes OpenAPI v3 structure.
func loadSchemasFromFS(fs afero.Fs) []spec3.OpenAPI {
	var schemas []spec3.OpenAPI

	// Look for schemas in the v3 directory structure
	// The cache has files at: api/openapi-spec/v3/*.json
	searchDirs := []string{
		"api/openapi-spec/v3",
		"v3",
		".",
	}

	for _, dir := range searchDirs {
		exists, err := afero.DirExists(fs, dir)
		if err != nil || !exists {
			continue
		}

		entries, err := afero.ReadDir(fs, dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			// Only process JSON files
			if filepath.Ext(entry.Name()) != ".json" {
				continue
			}

			filePath := filepath.Join(dir, entry.Name())
			schema, err := loadSchemaFile(fs, filePath)
			if err != nil {
				// Skip files that can't be loaded
				continue
			}

			schemas = append(schemas, schema)
		}
	}

	return schemas
}

// loadSchemaFile loads a single OpenAPI v3 schema file.
func loadSchemaFile(fs afero.Fs, filePath string) (spec3.OpenAPI, error) {
	data, err := afero.ReadFile(fs, filePath)
	if err != nil {
		return spec3.OpenAPI{}, errors.Wrapf(err, "cannot read file %q", filePath)
	}

	var schema spec3.OpenAPI
	if err := json.Unmarshal(data, &schema); err != nil {
		return spec3.OpenAPI{}, errors.Wrapf(err, "cannot parse OpenAPI JSON from %q", filePath)
	}

	return schema, nil
}
