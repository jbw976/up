// Copyright 2025 Upbound Inc.
// All rights reserved

//go:build integration
// +build integration

package apis

import (
	"context"
	"testing"

	"github.com/spf13/afero"
	"gotest.tools/v3/assert"

	"github.com/upbound/up/internal/schemas/generator"
	"github.com/upbound/up/internal/schemas/manager"
	"github.com/upbound/up/internal/schemas/runner"
)

func TestGenerateSchema(t *testing.T) {
	fs := afero.NewMemMapFs()
	m := manager.New(fs, generator.AllLanguages(), runner.NewRealSchemaRunner())

	err := GenerateSchema(context.Background(), m)
	assert.NilError(t, err)

	// This isn't an exhaustive list of files, but enough to be confident that
	// schema generation ran correctly for all languages.
	expected := []string{
		"kcl/models/kcl.mod",
		"kcl/models/k8s/apimachinery/pkg/apis/meta/v1",
		"python/models/io/k8s/apimachinery/pkg/apis/meta/__init__.py",
		"python/models/io/k8s/apimachinery/pkg/apis/meta/v1.py",
		"go/models/go.mod",
		"go/models/io/k8s/meta/v1/meta.go",
		"json/models/index.schema.json",
		"json/models/io-k8s-apimachinery-pkg-apis-meta-v1-ObjectMeta.schema.json",
	}

	for _, path := range expected {
		exists, err := afero.Exists(fs, path)
		assert.NilError(t, err)
		assert.Assert(t, exists, "expected path %q does not exist in output", path)
	}
}
