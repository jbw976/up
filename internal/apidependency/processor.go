// Copyright 2025 Upbound Inc.
// All rights reserved

// Package apidependency handles fetching and cache logic for project api-dependencies
package apidependency

import (
	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/git"
	"github.com/upbound/up/internal/schemas/manager"
	"github.com/upbound/up/pkg/apis/project/v2alpha1"
)

// Processor handles the complete lifecycle of API dependencies, creating
// appropriate schema sources that can fetch and provide resources directly.
type Processor struct {
	cloner       git.Cloner
	authProvider git.AuthProvider
	cache        Cache
}

// NewProcessor creates a new API dependency processor.
func NewProcessor(cloner git.Cloner, authProvider git.AuthProvider, cache Cache) *Processor {
	return &Processor{
		cloner:       cloner,
		authProvider: authProvider,
		cache:        cache,
	}
}

// Process creates the appropriate schema source based on the API dependency configuration.
func (p *Processor) Process(dep v2alpha1.APIDependencies) (manager.Source, error) {
	// Handle K8s type which always uses git
	if dep.Type == v2alpha1.APIDependencyTypeK8s {
		if dep.K8s == nil {
			return nil, errors.New("K8s configuration is required for K8s type")
		}
		// Convert K8s dependency to git dependency
		gitDep := v2alpha1.APIDependencies{
			Type: v2alpha1.APIDependencyTypeK8s,
			Git: &v2alpha1.APIGitReference{
				Repository: "https://github.com/kubernetes/kubernetes",
				Ref:        dep.K8s.Version,
				Path:       "api/openapi-spec",
			},
		}
		return p.createCachedSource(gitDep)
	}

	// For other types, check which source is configured
	if dep.Git != nil {
		return p.createCachedSource(dep)
	}

	if dep.HTTP != nil {
		return p.createCachedSource(dep)
	}

	return nil, errors.Errorf("no valid source configuration found for API dependency type %s", dep.Type)
}

// createCachedSource wraps the appropriate source with caching.
func (p *Processor) createCachedSource(dep v2alpha1.APIDependencies) (manager.Source, error) {
	var source manager.Source

	switch {
	case dep.Git != nil:
		source = manager.NewGitSource(dep, p.cloner, p.authProvider)
	case dep.HTTP != nil:
		source = manager.NewHTTPSource(dep)
	default:
		return nil, errors.New("no valid source configuration found")
	}

	// Wrap with cache if available
	if p.cache != nil {
		return NewCachedSource(source, p.cache, dep), nil
	}

	return source, nil
}
