// Copyright 2025 Upbound Inc.
// All rights reserved

package version

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestNewAvailable(t *testing.T) {
	type args struct {
		i      *Informer
		local  string
		remote string
	}

	type want struct {
		update bool
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NewVersionAvailable": {
			reason: "Should return true due to new version available.",
			args: args{
				i:      &Informer{},
				local:  "v0.0.1",
				remote: "v0.1.0",
			},
			want: want{
				update: true,
			},
		},
		"OlderVersionAvailable": {
			reason: "Should return false due to old version available.",
			args: args{
				i:      &Informer{},
				local:  "v0.1.0",
				remote: "v0.0.1",
			},
			want: want{
				update: false,
			},
		},
		"NewDevVersionLocal": {
			reason: "Should return false due to old version available.",
			args: args{
				i:      &Informer{},
				local:  "v0.7.0-rc.0.dirty",
				remote: "v0.6.0",
			},
			want: want{
				update: false,
			},
		},
		"DevVersionLocalNewVersionRemote": {
			reason: "Should return true due to new version available.",
			args: args{
				i:      &Informer{},
				local:  "v0.7.0-rc.0.dirty",
				remote: "v0.8.0",
			},
			want: want{
				update: true,
			},
		},
	}

	for _, tc := range cases {
		got := tc.args.i.newAvailable(tc.args.local, tc.args.remote)

		if diff := cmp.Diff(tc.want.update, got); diff != "" {
			t.Errorf("\n%s\nNewAvailable(...): -want err, +got err:\n%s", tc.reason, diff)
		}
	}
}

func TestGetCurrent(t *testing.T) {
	type args struct {
		i *Informer
	}

	type want struct {
		version string
		err     error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SuccessfulCall": {
			reason: "Should return expected version if successful.",
			args: args{
				i: &Informer{
					client: &mockClient{
						version: "v0.6.0",
					},
				},
			},
			want: want{
				version: "v0.6.0",
			},
		},
		"FailedCall": {
			reason: "Should return error due to failed call",
			args: args{
				i: &Informer{
					client: &mockClient{
						err: errors.New("boom"),
					},
				},
			},
			want: want{
				err: errors.New("boom"),
			},
		},
	}

	for _, tc := range cases {
		version, err := tc.args.i.getCurrent(context.Background())

		if diff := cmp.Diff(tc.want.version, version); diff != "" {
			t.Errorf("\n%s\nGetCurrent(...): -want err, +got err:\n%s", tc.reason, diff)
		}

		if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
			t.Errorf("\n%s\nGetCurrent(...): -want err, +got err:\n%s", tc.reason, diff)
		}
	}
}

type mockClient struct {
	version string
	err     error
}

func (m *mockClient) Do(_ *http.Request) (*http.Response, error) {
	return &http.Response{
		// NOTE(@tnthornton) the response from the real cli.upbound.io includes a `\n`
		Body: io.NopCloser(bytes.NewBufferString(fmt.Sprintf("%s\n", m.version))),
	}, m.err
}
