// Copyright 2025 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	"testing"

	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

func TestValidateCompositionTest(t *testing.T) {
	tests := []struct {
		name     string
		input    CompositionTest
		expected error
	}{
		{
			name: "ValidXROnly",
			input: CompositionTest{
				Spec: CompositionTestSpec{
					XR: runtime.RawExtension{Raw: []byte(`{}`)},
				},
			},
			expected: nil,
		},
		{
			name: "ValidXRPathOnly",
			input: CompositionTest{
				Spec: CompositionTestSpec{
					XRPath: "some/path",
				},
			},
			expected: nil,
		},
		{
			name: "InvalidXRTogetherWithXRPath",
			input: CompositionTest{
				Spec: CompositionTestSpec{
					XR:     runtime.RawExtension{Raw: []byte(`{}`)},
					XRPath: "some/path",
				},
			},
			expected: errors.New("only one of 'xr' or 'xrPath' may be specified"),
		},
		{
			name: "InvalidXRDTogetherWithXRDPath",
			input: CompositionTest{
				Spec: CompositionTestSpec{
					XRD:     runtime.RawExtension{Raw: []byte(`{}`)},
					XRDPath: "some/path",
				},
			},
			expected: errors.New("only one of 'xrd' or 'xrdPath' may be specified"),
		},
		{
			name: "InvalidCompositionTogetherWithCompositionPath",
			input: CompositionTest{
				Spec: CompositionTestSpec{
					Composition:     runtime.RawExtension{Raw: []byte(`{}`)},
					CompositionPath: "some/path",
				},
			},
			expected: errors.New("only one of 'composition' or 'compositionPath' may be specified"),
		},
		{
			name: "ValidEmptySpec",
			input: CompositionTest{
				Spec: CompositionTestSpec{},
			},
			expected: nil, // No strict requirement for a non-empty spec
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

func TestValidateCompositionTestSpec(t *testing.T) {
	tests := []struct {
		name     string
		input    CompositionTestSpec
		expected error
	}{
		{
			name: "ValidXROnly",
			input: CompositionTestSpec{
				XR: runtime.RawExtension{Raw: []byte(`{}`)},
			},
			expected: nil,
		},
		{
			name: "ValidXRPathOnly",
			input: CompositionTestSpec{
				XRPath: "some/path",
			},
			expected: nil,
		},
		{
			name: "InvalidXRTogetherWithXRPath",
			input: CompositionTestSpec{
				XR:     runtime.RawExtension{Raw: []byte(`{}`)},
				XRPath: "some/path",
			},
			expected: errors.New("only one of 'xr' or 'xrPath' may be specified"),
		},
		{
			name: "InvalidXRDTogetherWithXRDPath",
			input: CompositionTestSpec{
				XRD:     runtime.RawExtension{Raw: []byte(`{}`)},
				XRDPath: "some/path",
			},
			expected: errors.New("only one of 'xrd' or 'xrdPath' may be specified"),
		},
		{
			name: "InvalidCompositionTogetherWithCompositionPath",
			input: CompositionTestSpec{
				Composition:     runtime.RawExtension{Raw: []byte(`{}`)},
				CompositionPath: "some/path",
			},
			expected: errors.New("only one of 'composition' or 'compositionPath' may be specified"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := tt.input.validateCompositionTestSpec()
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
