// Copyright 2025 Upbound Inc.
// All rights reserved

package license

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/upbound/up/internal/http/mocks"
)

func TestGetAccessKey(t *testing.T) {
	errBoom := errors.New("boom")
	defaultURL, _ := url.Parse("https://test.com")
	bearerToken := "bearerToken"
	successToken := "TOKEN"
	successSig := "SIG"

	ctx := context.Background()

	type want struct {
		response *Response
	}

	cases := map[string]struct {
		reason    string
		provider  *DMV
		want      want
		clientErr error
		err       error
	}{
		"SuccessfulAuth": {
			reason: "A successful call to DMV should return the expected access key",
			provider: &DMV{
				client: &mocks.MockClient{
					DoFn: func(req *http.Request) (*http.Response, error) {
						return &http.Response{
							StatusCode: http.StatusOK,
							Body:       io.NopCloser(bytes.NewReader([]byte(fmt.Sprintf(`{"key": "%s", "signature": "%s"}`, successToken, successSig)))),
						}, nil
					},
				},
				endpoint: defaultURL,
			},
			want: want{
				response: &Response{
					AccessKey: successToken,
					Signature: successSig,
				},
			},
		},

		"ErrAuthFailed": {
			reason: "If call to dmv fails an error should be returned.",
			provider: &DMV{
				client: &mocks.MockClient{
					DoFn: func(req *http.Request) (*http.Response, error) {
						return nil, errBoom
					},
				},
				endpoint: defaultURL,
			},
			err: errors.Wrap(errBoom, errGetAccessKey),
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			resp, err := tc.provider.GetAccessKey(ctx, bearerToken, "version")
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGetAccessKey(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.response, resp); diff != "" {
				t.Errorf("\n%s\nGetAccessKey(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
