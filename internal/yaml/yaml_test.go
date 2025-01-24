// Copyright 2025 Upbound Inc.
// All rights reserved

package yaml

import (
	"testing"
	"time"

	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type objectWithoutMetadata struct {
	FieldA string `json:"fieldA"`
	FieldB string `jsoN:"fieldB"`
}

type metadataWithoutCreationTimestamp struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type objectWithoutCreationTimestamp struct {
	Metadata metadataWithoutCreationTimestamp `json:"metadata"`
}

type objectWithCreationTimestamp struct {
	metav1.ObjectMeta `json:"metadata"`
}

type objectWithNestedFields struct {
	metav1.ObjectMeta `json:"metadata"`
	TopLevel          *objectWithoutMetadata `yaml:"topLevel"`
}

func TestMarshal(t *testing.T) {
	tcs := map[string]struct {
		input        any
		opts         []MarshalOption
		expectedYAML string
	}{
		"NoMetadata": {
			input: &objectWithoutMetadata{
				FieldA: "hello",
				FieldB: "world",
			},
			expectedYAML: `fieldA: hello
fieldB: world
`,
		},
		"NoTimestamp": {
			input: &objectWithoutCreationTimestamp{
				Metadata: metadataWithoutCreationTimestamp{
					Name:      "hello",
					Namespace: "world",
				},
			},
			expectedYAML: `metadata:
  name: hello
  namespace: world
`,
		},
		"NilTimestamp": {
			input: &objectWithCreationTimestamp{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hello",
					Namespace: "world",
				},
			},
			expectedYAML: `metadata:
  name: hello
  namespace: world
`,
		},
		"NonNilTimestamp": {
			input: &objectWithCreationTimestamp{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "hello",
					Namespace:         "world",
					CreationTimestamp: metav1.Date(2024, 11, 7, 12, 13, 14, 0, time.UTC),
				},
			},
			expectedYAML: `metadata:
  creationTimestamp: "2024-11-07T12:13:14Z"
  name: hello
  namespace: world
`,
		},
		"NonPointer": {
			input: objectWithCreationTimestamp{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "hello",
					Namespace:         "world",
					CreationTimestamp: metav1.Date(2024, 11, 7, 12, 13, 14, 0, time.UTC),
				},
			},
			expectedYAML: `metadata:
  creationTimestamp: "2024-11-07T12:13:14Z"
  name: hello
  namespace: world
`,
		},
		"RemoveField": {
			input: objectWithNestedFields{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hello",
					Namespace: "world",
				},
				TopLevel: &objectWithoutMetadata{
					FieldA: "hello",
					FieldB: "world",
				},
			},
			opts: []MarshalOption{
				RemoveField("topLevel.fieldA"),
			},
			expectedYAML: `metadata:
  name: hello
  namespace: world
topLevel:
  fieldB: world
`,
		},
		"RemoveFieldIfNil": {
			input: objectWithNestedFields{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hello",
					Namespace: "world",
				},
			},
			opts: []MarshalOption{
				RemoveFieldIfNil("topLevel"),
			},
			expectedYAML: `metadata:
  name: hello
  namespace: world
`,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			bs, err := Marshal(tc.input, tc.opts...)
			assert.NilError(t, err)
			assert.Equal(t, string(bs), tc.expectedYAML)
		})
	}
}
