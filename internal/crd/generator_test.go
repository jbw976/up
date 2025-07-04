// Copyright 2025 Upbound Inc.
// All rights reserved

package crd

import (
	"testing"

	"github.com/spf13/afero"
	"gotest.tools/v3/assert"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/upbound/up/internal/yaml"

	_ "embed"
)

//go:embed testdata/claimable-xrd.yaml
var claimableXRDBytes []byte

//go:embed testdata/unclaimable-xrd.yaml
var unclaimableXRDBytes []byte

func TestProcessXRD(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		xrdBytes []byte

		expectedXRKind     string
		expectedXRListKind string

		expectedClaimKind     string
		expectedClaimListKind string
	}{
		"ClaimableXRD": {
			xrdBytes:              claimableXRDBytes,
			expectedXRKind:        "XStorageBucket",
			expectedXRListKind:    "XStorageBucketList",
			expectedClaimKind:     "StorageBucket",
			expectedClaimListKind: "StorageBucketList",
		},
		"UnclaimableXRD": {
			xrdBytes:           unclaimableXRDBytes,
			expectedXRKind:     "XInternalBucket",
			expectedXRListKind: "XInternalBucketList",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			outFS := afero.NewMemMapFs()
			claimPath, xrPath, err := ProcessXRD(outFS, tc.xrdBytes, "output", "/")
			assert.NilError(t, err)

			if xrPath != "" {
				assert.Assert(t, tc.expectedXRKind != "", "unexpected XR CRD generated")
				xrBytes, err := afero.ReadFile(outFS, xrPath)
				assert.NilError(t, err)

				var xrCRD extv1.CustomResourceDefinition
				err = yaml.Unmarshal(xrBytes, &xrCRD)
				assert.NilError(t, err)

				assert.Equal(t, xrCRD.Spec.Names.Kind, tc.expectedXRKind)
				assert.Equal(t, xrCRD.Spec.Names.ListKind, tc.expectedXRListKind)
			}

			if claimPath != "" {
				assert.Assert(t, tc.expectedClaimKind != "", "unexpected claim CRD generated")
				claimBytes, err := afero.ReadFile(outFS, claimPath)
				assert.NilError(t, err)

				var claimCRD extv1.CustomResourceDefinition
				err = yaml.Unmarshal(claimBytes, &claimCRD)
				assert.NilError(t, err)

				assert.Equal(t, claimCRD.Spec.Names.Kind, tc.expectedClaimKind)
				assert.Equal(t, claimCRD.Spec.Names.ListKind, tc.expectedClaimListKind)
			}
		})
	}
}
