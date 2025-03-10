// Copyright 2025 Upbound Inc.
// All rights reserved

package kcl

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestFormatKclImportPath(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		inputPath       string
		expectedImport  string
		expectedAlias   string
		existingAliases map[string]bool
	}{
		"BasicCase": {
			inputPath:       "models/io/upbound/aws/v1alpha1",
			expectedImport:  "models.io.upbound.aws.v1alpha1",
			expectedAlias:   "awsv1alpha1",
			existingAliases: map[string]bool{},
		},
		"AliasConflict": {
			inputPath:      "models/io/upbound/platformref/aws/v1alpha1",
			expectedImport: "models.io.upbound.platformref.aws.v1alpha1",
			expectedAlias:  "platformrefawsv1alpha1",
			existingAliases: map[string]bool{
				"awsv1alpha1": true, // Simulate alias conflict
			},
		},
		"MoreImports": {
			inputPath:      "models/io/upbound/aws/ec2/v1beta1",
			expectedImport: "models.io.upbound.aws.ec2.v1beta1",
			expectedAlias:  "ec2v1beta1",
			existingAliases: map[string]bool{
				"awsv1alpha1": true,
				"s3v1alpha1":  true,
			},
		},
		"NoModelsKeyword": {
			inputPath:       "random/path/without/models",
			expectedImport:  "",
			expectedAlias:   "",
			existingAliases: map[string]bool{},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			importPath, alias := FormatKclImportPath(tc.inputPath, tc.existingAliases)
			assert.Equal(t, importPath, tc.expectedImport)
			assert.Equal(t, alias, tc.expectedAlias)
		})
	}
}
