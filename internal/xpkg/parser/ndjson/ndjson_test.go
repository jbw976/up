// Copyright 2025 Upbound Inc.
// All rights reserved

package ndjson

import (
	"bufio"
	"bytes"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestJSONReader(t *testing.T) {
	type args struct {
		doc []byte
	}

	type want struct {
		docsRead int
		err      error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SingleLineJSON": {
			reason: "Should successfully read single line JSON file.",
			args: args{
				doc: []byte(`{"type":"crd"}`),
			},
			want: want{
				docsRead: 1,
			},
		},
		"MultiNDJSONObjects": {
			reason: "Should successfully read multi line (newline delimited) JSON file.",
			args: args{
				doc: []byte(`{"thing":"1"}
		{"thing":"2"}
		{"thing":"3"}`),
			},
			want: want{
				docsRead: 3,
			},
		},
		"MultiNDJSONObjectsBlankLine": {
			reason: "Should successfully read multi line (newline delimited) JSON file with blank line.",
			args: args{
				doc: []byte(`{"thing":"1"}
		{"thing":"2"}

		{"thing":"3"}`),
			},
			want: want{
				docsRead: 3,
			},
		},
		"MultiNDJSONObjectsTrailingLine": {
			reason: "Should successfully read multi line (newline delimited) JSON file with blank line and trailing line.",
			args: args{
				doc: []byte(`{"thing":"1"}
{"thing":"2"}

{"thing":"3"}





`),
			},
			want: want{
				docsRead: 3,
			},
		},
		"EmptyDoc": {
			reason: "Should successfully read an empty file.",
			args: args{
				doc: []byte(nil),
			},
			want: want{
				docsRead: 0,
			},
		},
	}

	for _, tc := range cases {
		r := NewReader(bufio.NewReader(bytes.NewReader(tc.args.doc)))
		docsRead := 0

		var err error
		for {
			b, e := r.Read()
			if len(b) != 0 {
				docsRead++
			}

			if e != nil {
				err = e
				break
			}
		}
		if diff := cmp.Diff(tc.want.docsRead, docsRead); diff != "" {
			t.Errorf("\n%s\nNDJSONReader(...): -want err, +got err:\n%s", tc.reason, diff)
		}

		// we expect to eventually see an EOF, test if we saw other errors
		if !errors.Is(err, io.EOF) {
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nNDJSONReader(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		}
	}
}
