// Copyright 2025 Upbound Inc.
// All rights reserved

package v1alpha1

import (
	"testing"

	"gotest.tools/v3/assert"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
)

func TestValidateOperationTest(t *testing.T) {
	tests := []struct {
		name     string
		input    OperationTest
		expected error
	}{
		{
			name: "ValidRequiredResourcesOnly",
			input: OperationTest{
				Spec: OperationTestSpec{
					RequiredResources: []runtime.RawExtension{{Raw: []byte(`{}`)}},
				},
			},
			expected: nil,
		},
		{
			name: "ValidRequiredResourcesPathOnly",
			input: OperationTest{
				Spec: OperationTestSpec{
					RequiredResourcesPath: "some/path",
				},
			},
			expected: nil,
		},
		{
			name: "InvalidRequiredResourcesTogetherWithRequiredResourcesPath",
			input: OperationTest{
				Spec: OperationTestSpec{
					RequiredResources:     []runtime.RawExtension{{Raw: []byte(`{}`)}},
					RequiredResourcesPath: "some/path",
				},
			},
			expected: errors.New("only one of 'requiredResources' or 'requiredResourcesPath' may be specified"),
		},
		{
			name: "ValidEmptySpec",
			input: OperationTest{
				Spec: OperationTestSpec{},
			},
			expected: nil, // No strict requirement for a non-empty spec
		},
		{
			name: "ValidWithContext",
			input: OperationTest{
				Spec: OperationTestSpec{
					Context: map[string]runtime.RawExtension{
						"key1": {Raw: []byte(`{"data": "value"}`)},
					},
				},
			},
			expected: nil,
		},
		{
			name: "ValidWithAssertResources",
			input: OperationTest{
				Spec: OperationTestSpec{
					AssertResources: []runtime.RawExtension{{Raw: []byte(`{}`)}},
				},
			},
			expected: nil,
		},
		{
			name: "ValidWithFunctionCredentialsPath",
			input: OperationTest{
				Spec: OperationTestSpec{
					FunctionCredentialsPath: "credentials/path",
				},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.expected == nil {
				assert.NilError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.expected.Error())
			}
		})
	}
}

func TestValidateOperationTestSpec(t *testing.T) {
	tests := []struct {
		name     string
		input    OperationTestSpec
		expected error
	}{
		{
			name: "ValidRequiredResourcesOnly",
			input: OperationTestSpec{
				RequiredResources: []runtime.RawExtension{{Raw: []byte(`{}`)}},
			},
			expected: nil,
		},
		{
			name: "ValidRequiredResourcesPathOnly",
			input: OperationTestSpec{
				RequiredResourcesPath: "some/path",
			},
			expected: nil,
		},
		{
			name: "InvalidRequiredResourcesTogetherWithRequiredResourcesPath",
			input: OperationTestSpec{
				RequiredResources:     []runtime.RawExtension{{Raw: []byte(`{}`)}},
				RequiredResourcesPath: "some/path",
			},
			expected: errors.New("only one of 'requiredResources' or 'requiredResourcesPath' may be specified"),
		},
		{
			name:     "ValidEmptySpec",
			input:    OperationTestSpec{},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := tt.input.validateOperationTestSpec()
			if tt.expected == nil {
				assert.Equal(t, len(errs), 0)
			} else {
				found := false
				for _, err := range errs {
					if err.Error() == tt.expected.Error() {
						found = true
						break
					}
				}
				assert.Assert(t, found, "expected error: %v", tt.expected)
			}
		})
	}
}
