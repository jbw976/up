// Copyright 2025 Upbound Inc.
// All rights reserved

package schemagenerator

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/invopop/jsonschema"
	"github.com/spf13/afero"
	"gotest.tools/v3/assert"
)

func TestGenerateJSON(t *testing.T) {
	inputFS := afero.NewBasePathFs(afero.FromIOFS{FS: testdataFS}, "testdata")
	schemaFS, err := GenerateSchemaJSON(context.Background(), inputFS, nil, nil)
	assert.NilError(t, err)

	expectedFiles := []string{
		"models/io-k8s-apimachinery-pkg-apis-meta-v1-DeleteOptions.schema.json",
		"models/io-k8s-apimachinery-pkg-apis-meta-v1-FieldsV1.schema.json",
		"models/io-k8s-apimachinery-pkg-apis-meta-v1-ListMeta.schema.json",
		"models/io-k8s-apimachinery-pkg-apis-meta-v1-ManagedFieldsEntry.schema.json",
		"models/io-k8s-apimachinery-pkg-apis-meta-v1-ObjectMeta.schema.json",
		"models/io-k8s-apimachinery-pkg-apis-meta-v1-OwnerReference.schema.json",
		"models/io-k8s-apimachinery-pkg-apis-meta-v1-Patch.schema.json",
		"models/io-k8s-apimachinery-pkg-apis-meta-v1-Preconditions.schema.json",
		"models/io-k8s-apimachinery-pkg-apis-meta-v1-StatusCause.schema.json",
		"models/io-k8s-apimachinery-pkg-apis-meta-v1-StatusDetails.schema.json",
		"models/io-k8s-apimachinery-pkg-apis-meta-v1-Status.schema.json",
		"models/io-k8s-apimachinery-pkg-apis-meta-v1-Time.schema.json",
		"models/co-acme-platform-v1alpha1-AccountScaffold.schema.json",
		"models/co-acme-platform-v1alpha1-AccountScaffoldList.schema.json",
		"models/co-acme-platform-v1alpha1-XAccountScaffold.schema.json",
		"models/co-acme-platform-v1alpha1-XAccountScaffoldList.schema.json",
	}

	for _, path := range expectedFiles {
		exists, err := afero.Exists(schemaFS, path)
		assert.NilError(t, err)
		assert.Assert(t, exists, "expected model file %s does not exist", path)

		contents, err := afero.ReadFile(schemaFS, path)
		assert.NilError(t, err)

		var schema jsonschema.Schema
		err = json.Unmarshal(contents, &schema)
		assert.NilError(t, err)
	}
}
