// Copyright 2025 Upbound Inc.
// All rights reserved

package project

import (
	"embed"
	"testing"

	"github.com/spf13/afero"
	"gotest.tools/v3/assert"

	xpextv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	opsv1alpha1 "github.com/crossplane/crossplane/apis/ops/v1alpha1"

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
	err = Move(t.Context(), proj, projFS, newRepo)
	assert.NilError(t, err)

	moved, err := Parse(projFS, "upbound.yaml")
	assert.NilError(t, err)
	assert.Equal(t, moved.Spec.Repository, newRepo)

	// Check that the composition pipeline was updated.
	bs, err := afero.ReadFile(projFS, "apis/xstoragebuckets/composition.yaml")
	assert.NilError(t, err)

	var comp xpextv1.Composition
	err = yaml.Unmarshal(bs, &comp)
	assert.NilError(t, err)
	assert.Equal(t, comp.Spec.Pipeline[0].FunctionRef.Name, "other-org-other-repocompose-bucket-kcl")

	// Check that the oneshot operation pipeline was updated.
	bs, err = afero.ReadFile(projFS, "operations/my-operation/operation.yaml")
	assert.NilError(t, err)

	var oper opsv1alpha1.Operation
	err = yaml.Unmarshal(bs, &oper)
	assert.NilError(t, err)
	assert.Equal(t, oper.Spec.Pipeline[0].FunctionRef.Name, "other-org-other-repomy-op-fn")

	// Check that the cron operation pipeline was updated.
	bs, err = afero.ReadFile(projFS, "operations/my-cron-operation/operation.yaml")
	assert.NilError(t, err)

	var coper opsv1alpha1.CronOperation
	err = yaml.Unmarshal(bs, &coper)
	assert.NilError(t, err)
	assert.Equal(t, coper.Spec.OperationTemplate.Spec.Pipeline[0].FunctionRef.Name, "other-org-other-repomy-op-fn")
}
