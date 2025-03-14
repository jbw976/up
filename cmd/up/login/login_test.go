// Copyright 2025 Upbound Inc.
// All rights reserved

package login

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"testing"
	"testing/iotest"

	"github.com/golang-jwt/jwt"
	"github.com/google/go-cmp/cmp"
	"github.com/pterm/pterm"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/upbound/up/internal/http/mocks"
	inputmocks "github.com/upbound/up/internal/input/mocks"
	"github.com/upbound/up/internal/profile"
	"github.com/upbound/up/internal/upbound"
)

func TestRun(t *testing.T) {
	errBoom := errors.New("boom")
	defaultURL, _ := url.Parse("https://test.com")
	// NOTE: This *must* be empty, otherwise the test will try to open a
	// browser.
	accountsURL, _ := url.Parse("")

	cases := map[string]struct {
		reason string
		cmd    *LoginCmd
		ctx    *upbound.Context
		err    error
	}{
		"ErrLoginFailed": {
			reason: "If Upbound Cloud endpoint is ",
			cmd: &LoginCmd{
				client: &mocks.MockClient{
					DoFn: func(_ *http.Request) (*http.Response, error) {
						return nil, errBoom
					},
				},
				Username: "cool-user",
				Password: "cool-pass",
			},
			ctx: &upbound.Context{
				APIEndpoint:      defaultURL,
				AccountsEndpoint: accountsURL,
			},
			err: errors.Wrap(errBoom, errLoginFailed),
		},
		"ErrCannotLaunchBrowser": {
			reason: "non-interactive terminals won't prompt",
			cmd: &LoginCmd{
				client: &mocks.MockClient{
					DoFn: func(_ *http.Request) (*http.Response, error) {
						return nil, errBoom
					},
				},
				prompter: &inputmocks.MockPrompter{},
				Username: "",
			},
			ctx: &upbound.Context{
				APIEndpoint:      defaultURL,
				AccountsEndpoint: accountsURL,
			},
			err: errors.Wrap(errors.New(inputmocks.ErrCannotPrompt), errLoginFailed),
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if diff := cmp.Diff(tc.err, tc.cmd.Run(context.TODO(), pterm.DefaultBasicText.WithWriter(io.Discard), tc.ctx), test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRun(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestConstructAuth(t *testing.T) {
	type args struct {
		username string
		token    string
		password string
	}
	type want struct {
		pType profile.TokenType
		auth  *auth
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
		err    error
	}{
		"SuccessfulUser": {
			reason: "Providing a valid id and password should return a valid auth request.",
			args: args{
				username: "cool-user",
				password: "cool-password",
			},
			want: want{
				pType: profile.TokenTypeUser,
				auth: &auth{
					ID:       "cool-user",
					Password: "cool-password",
					Remember: true,
				},
			},
		},
		"SuccessfulToken": {
			reason: "Providing a valid id and token should return a valid auth request.",
			args: args{
				token: "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJPbmxpbmUgSldUIEJ1aWxkZXIiLCJpYXQiOjE2MTg1MTc5NDMsImV4cCI6MTY1MDA1Mzk0MywiYXVkIjoiaHR0cHM6Ly9kYW5pZWxtYW5ndW0uY29tIiwic3ViIjoiZ2VvcmdlZGFuaWVsbWFuZ3VtQGdtYWlsLmNvbSIsIkpUSSI6Imhhc2hlZGRhbiJ9.zI42wXvwDHiATx9ycECz7JyATTn9P07wN-TRXvtCGcM",
			},
			want: want{
				pType: profile.TokenTypePAT,
				auth: &auth{
					ID:       "hasheddan",
					Password: "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJPbmxpbmUgSldUIEJ1aWxkZXIiLCJpYXQiOjE2MTg1MTc5NDMsImV4cCI6MTY1MDA1Mzk0MywiYXVkIjoiaHR0cHM6Ly9kYW5pZWxtYW5ndW0uY29tIiwic3ViIjoiZ2VvcmdlZGFuaWVsbWFuZ3VtQGdtYWlsLmNvbSIsIkpUSSI6Imhhc2hlZGRhbiJ9.zI42wXvwDHiATx9ycECz7JyATTn9P07wN-TRXvtCGcM",
					Remember: true,
				},
			},
		},
		"SuccessfulTokenIgnorePassword": {
			reason: "Providing a valid id and token should return a valid auth request without extraneous password.",
			args: args{
				token:    "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJPbmxpbmUgSldUIEJ1aWxkZXIiLCJpYXQiOjE2MTg1MTc5NDMsImV4cCI6MTY1MDA1Mzk0MywiYXVkIjoiaHR0cHM6Ly9kYW5pZWxtYW5ndW0uY29tIiwic3ViIjoiZ2VvcmdlZGFuaWVsbWFuZ3VtQGdtYWlsLmNvbSIsIkpUSSI6Imhhc2hlZGRhbiJ9.zI42wXvwDHiATx9ycECz7JyATTn9P07wN-TRXvtCGcM",
				password: "forget-about-me",
			},
			want: want{
				pType: profile.TokenTypePAT,
				auth: &auth{
					ID:       "hasheddan",
					Password: "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJPbmxpbmUgSldUIEJ1aWxkZXIiLCJpYXQiOjE2MTg1MTc5NDMsImV4cCI6MTY1MDA1Mzk0MywiYXVkIjoiaHR0cHM6Ly9kYW5pZWxtYW5ndW0uY29tIiwic3ViIjoiZ2VvcmdlZGFuaWVsbWFuZ3VtQGdtYWlsLmNvbSIsIkpUSSI6Imhhc2hlZGRhbiJ9.zI42wXvwDHiATx9ycECz7JyATTn9P07wN-TRXvtCGcM",
					Remember: true,
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			auth, profType, err := constructAuth(tc.args.username, tc.args.token, tc.args.password)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nconstructAuth(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.auth, auth); diff != "" {
				t.Errorf("\n%s\nconstructAuth(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.pType, profType); diff != "" {
				t.Errorf("\n%s\nconstructAuth(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestParseID(t *testing.T) {
	type args struct {
		username string
		token    string
	}
	type want struct {
		id    string
		pType profile.TokenType
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
		err    error
	}{
		"ErrorInvalidToken": {
			reason: "If token is not a valid JWT an error should be returned.",
			args: args{
				token: "invalid",
			},
			err: jwt.NewValidationError("token contains an invalid number of segments", jwt.ValidationErrorMalformed),
		},
		"ErrorNoClaimID": {
			reason: "If token does not contain an ID an error should be returned.",
			args: args{
				token: "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJPbmxpbmUgSldUIEJ1aWxkZXIiLCJpYXQiOjE2MTg1MTc1NDQsImV4cCI6MTY1MDA1MzU0NCwiYXVkIjoiaHR0cHM6Ly9kYW5pZWxtYW5ndW0uY29tIiwic3ViIjoiZ2VvcmdlZGFuaWVsbWFuZ3VtQGdtYWlsLmNvbSIsIkZpcnN0IjoiRGFuIiwiU3VybmFtZSI6Ik1hbmd1bSJ9.8F4mgY5-lpt2KmGx7Z8yeSorfs-WRgdJmCq8mCcrxZQ",
			},
			err: errors.New(errNoIDInToken),
		},
		"SuccessfulToken": {
			reason: "Providing a valid token should return a valid auth request.",
			args: args{
				token: "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJPbmxpbmUgSldUIEJ1aWxkZXIiLCJpYXQiOjE2MTg1MTc5NDMsImV4cCI6MTY1MDA1Mzk0MywiYXVkIjoiaHR0cHM6Ly9kYW5pZWxtYW5ndW0uY29tIiwic3ViIjoiZ2VvcmdlZGFuaWVsbWFuZ3VtQGdtYWlsLmNvbSIsIkpUSSI6Imhhc2hlZGRhbiJ9.zI42wXvwDHiATx9ycECz7JyATTn9P07wN-TRXvtCGcM",
			},
			want: want{
				id:    "hasheddan",
				pType: profile.TokenTypePAT,
			},
		},
		"SuccessfulTokenTypeRobot": {
			reason: "Providing a valid robot token should return a TokenTypeRobot.",
			args: args{
				token: "eyJhbGciOiJSUzI1NiIsImtpZCI6IlRfIiwidHlwIjoiSldUIn0.eyJhdWQiOiJ1cGJvdW5kLmlvIiwiZXhwIjoyMDU3MzEwOTE5LCJqdGkiOiIyOTg1YTMyOC0xZDg0LTQ3ZjMtYjVkNC0wYTBkNjhhOGJjMDQiLCJpc3MiOiJodHRwczovL2FwaS51cGJvdW5kLmlvL3YxIiwic3ViIjoicm9ib3R8ODZlOWEzNmQtOWY2Yy00YWQxLTkxYjEtZjljODdiMDA3OGZjIn0.YPt5sbCN1uiV2K8LdnVNOjnfhkvFpZ4RtcynCJR5mkxx0bJHV1w8kC0ZrYe7e5qNxeU_88vZS7qamoWmNRzn6bvI59h05RFzPaP1tIehlZ2EWRGmlM7wIZAlsfM-kanSnZQGwMsmsqAzn-54-G-RxiKC4dD552Go-lFlp2rkDz273wIdZvO10ocqoGNzvtuTcnYAHhLfafgEBPWsjTP09x_Mf-u3eA8t0nL2aPH9WrEJNl66D5F4Ex0NMQFW60ZWTdgCQRio6ZcYHUX3hL6DSljAEpTedIRzwk8R8R-uAohoT62WXnP4BaMxpnrIQPzBAZyAIhWZqeyTnWmtdb9Imw",
			},
			want: want{
				id:    "2985a328-1d84-47f3-b5d4-0a0d68a8bc04",
				pType: profile.TokenTypeRobot,
			},
		},
		"Successful": {
			reason: "Providing a username should return a valid auth request.",
			args: args{
				username: "cool-user",
			},
			want: want{
				id:    "cool-user",
				pType: profile.TokenTypeUser,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			id, pType, err := parseID(tc.args.username, tc.args.token)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nparseID(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.id, id, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nparseID(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.pType, pType, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nparseID(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestExtractSession(t *testing.T) {
	errBoom := errors.New("boom")
	cook := http.Cookie{
		Name:  "SID",
		Value: "cool-session",
	}
	type args struct {
		res  *http.Response
		name string
	}
	cases := map[string]struct {
		reason string
		args   args
		want   string
		err    error
	}{
		"ErrorNoCookieFailReadBody": {
			reason: "Should return an error if cookie does not exist and we fail to read body.",
			args: args{
				res: &http.Response{
					Body: io.NopCloser(iotest.ErrReader(errBoom)),
				},
			},
			err: errors.Wrap(errBoom, errReadBody),
		},
		"ErrorNoCookie": {
			reason: "Should return an error if cookie does not exist.",
			args: args{
				res: &http.Response{
					Body: io.NopCloser(bytes.NewBufferString("unauthorized")),
				},
			},
			err: errors.Errorf(errParseCookieFmt, "unauthorized"),
		},
		"Successful": {
			reason: "Should return cookie value if it exists.",
			args: args{
				res: &http.Response{
					Header: http.Header{"Set-Cookie": []string{cook.String()}},
				},
				name: cook.Name,
			},
			want: cook.Value,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			session, err := extractSession(tc.args.res, tc.args.name)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nextractSession(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want, session); diff != "" {
				t.Errorf("\n%s\nextractSession(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestIsEmail(t *testing.T) {
	cases := map[string]struct {
		reason string
		user   string
		want   bool
	}{
		"UserIsEmail": {
			reason: "Should return true if username is an email address.",
			user:   "dan@upbound.io",
			want:   true,
		},
		"NotEmail": {
			reason: "Should return false if username is not an email address.",
			user:   "hasheddan",
			want:   false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := isEmail(tc.user)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nisEmail(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
