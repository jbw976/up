// Copyright 2025 Upbound Inc.
// All rights reserved

package project

import (
	"context"
	"embed"
	"testing"

	"github.com/spf13/afero"
	"gotest.tools/v3/assert"

	xpextv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"

	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/internal/yaml"
)

//go:embed testdata/example-project/**
var exampleProject embed.FS

func TestMove(t *testing.T) {
	projFS := afero.NewBasePathFs(
		afero.FromIOFS{FS: exampleProject},
		"testdata/example-project",
	)
	projFS = filesystem.MemOverlay(projFS)

	proj, err := Parse(projFS, "upbound.yaml")
	assert.NilError(t, err)
	proj.Default()

	const newRepo = "xpkg.upbound.io/other-org/other-repo"
	err = Move(context.Background(), proj, projFS, newRepo)
	assert.NilError(t, err)

	moved, err := Parse(projFS, "upbound.yaml")
	assert.NilError(t, err)
	assert.Equal(t, moved.Spec.Repository, newRepo)

	bs, err := afero.ReadFile(projFS, "apis/xstoragebuckets/composition.yaml")
	assert.NilError(t, err)

	var comp xpextv1.Composition
	err = yaml.Unmarshal(bs, &comp)
	assert.NilError(t, err)
	assert.Equal(t, comp.Spec.Pipeline[0].FunctionRef.Name, "other-org-other-repocompose-bucket-kcl")
}
