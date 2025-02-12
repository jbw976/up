// Copyright 2025 Upbound Inc.
// All rights reserved

package json

import (
	"bytes"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/upbound/up/internal/usage/model"
)

var errWriteFailed = fmt.Errorf("write failed")

type errWriter struct{}

func (w *errWriter) Write(p []byte) (int, error) {
	return 0, errWriteFailed
}

func TestNewMXPGVKEventEncoder(t *testing.T) {
	type args struct {
		writer io.Writer
	}
	type want struct {
		err error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Success": {
			reason: "An encoder can be successfully created.",
			args: args{
				writer: &bytes.Buffer{},
			},
			want: want{
				err: nil,
			},
		},
		"ErrOnWrite": {
			reason: "Creating an encoder with a writer that returns an error on write returns an error.",
			args: args{
				writer: &errWriter{},
			},
			want: want{
				err: errWriteFailed,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := NewMXPGVKEventEncoder(tc.args.writer)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nNewMXPGVKEventEncoder(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestMXPGVKEventEncoder(t *testing.T) {
	type args struct {
		events []model.MXPGVKEvent
	}
	type want struct {
		bytes []byte
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoEvents": {
			reason: "Encoder is closed without writing any events.",
			args: args{
				events: []model.MXPGVKEvent{},
			},
			want: want{
				bytes: []byte("[\n]\n"),
			},
		},
		"OneEvent": {
			reason: "Encoder is closed after writing one event.",
			args: args{
				events: []model.MXPGVKEvent{{}},
			},
			want: want{
				bytes: []byte(`[
{"name":"","tags":{"customresource_group":"","customresource_version":"","customresource_kind":"","upbound_account":"","mxp_id":""},"timestamp":"0001-01-01T00:00:00Z","timestamp_end":"0001-01-01T00:00:00Z","value":0}
]
`),
			},
		},
		"MultipleEvents": {
			reason: "Encoder is closed after writing multiple events.",
			args: args{
				events: []model.MXPGVKEvent{
					{
						Name:         "test_event",
						Timestamp:    time.Date(2006, 5, 4, 3, 2, 1, 0, time.UTC),
						TimestampEnd: time.Date(2006, 5, 4, 3, 3, 1, 0, time.UTC),
						Value:        5.0,
						Tags: model.MXPGVKEventTags{
							Group:          "example.com",
							Version:        "v1",
							Kind:           "things",
							UpboundAccount: "test-account",
							MXPID:          "test-mxpid",
						},
					},
					{
						Name:         "test_event",
						Timestamp:    time.Date(2006, 5, 4, 3, 2, 1, 0, time.UTC),
						TimestampEnd: time.Date(2006, 5, 4, 3, 3, 1, 0, time.UTC),
						Value:        10.0,
						Tags: model.MXPGVKEventTags{
							Group:          "example.com",
							Version:        "v1",
							Kind:           "foos",
							UpboundAccount: "test-account",
							MXPID:          "test-mxpid",
						},
					},
					{
						Name:         "test_event",
						Timestamp:    time.Date(2006, 5, 4, 3, 2, 1, 0, time.UTC),
						TimestampEnd: time.Date(2006, 5, 4, 3, 3, 1, 0, time.UTC),
						Value:        8.0,
						Tags: model.MXPGVKEventTags{
							Group:          "example.com",
							Version:        "v1alpha1",
							Kind:           "bars",
							UpboundAccount: "test-account",
							MXPID:          "test-mxpid",
						},
					},
				},
			},
			want: want{
				bytes: []byte(`[
{"name":"test_event","tags":{"customresource_group":"example.com","customresource_version":"v1","customresource_kind":"things","upbound_account":"test-account","mxp_id":"test-mxpid"},"timestamp":"2006-05-04T03:02:01Z","timestamp_end":"2006-05-04T03:03:01Z","value":5},
{"name":"test_event","tags":{"customresource_group":"example.com","customresource_version":"v1","customresource_kind":"foos","upbound_account":"test-account","mxp_id":"test-mxpid"},"timestamp":"2006-05-04T03:02:01Z","timestamp_end":"2006-05-04T03:03:01Z","value":10},
{"name":"test_event","tags":{"customresource_group":"example.com","customresource_version":"v1alpha1","customresource_kind":"bars","upbound_account":"test-account","mxp_id":"test-mxpid"},"timestamp":"2006-05-04T03:02:01Z","timestamp_end":"2006-05-04T03:03:01Z","value":8}
]
`),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			buf := bytes.Buffer{}
			e, err := NewMXPGVKEventEncoder(&buf)
			if err != nil {
				diff := cmp.Diff(nil, err, test.EquateErrors())
				t.Errorf("\n%s\nNewMXPGVKEventEncoder(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			for _, event := range tc.args.events {
				err := e.Encode(event)
				if diff := cmp.Diff(nil, err, test.EquateErrors()); diff != "" {
					t.Errorf("\n%s\nMXPGVKEventEncoder.Encode(): -want err, +got err:\n%s", tc.reason, diff)
				}
			}
			err = e.Close()
			if diff := cmp.Diff(nil, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nMXPGVKEventEncoder.Close(): -want err, +got err:\n%s", tc.reason, diff)
			}

			got := buf.Bytes()
			if diff := cmp.Diff(tc.want.bytes, got); diff != "" {
				t.Errorf("\n%s\nMXPGVKEventEncoder: -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
