// Copyright 2025 Upbound Inc.
// All rights reserved

package credhelper

import (
	"testing"
	"time"

	"github.com/docker/docker-credential-helpers/credentials"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/profile"
)

func testToken(t *testing.T, exp time.Time) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(exp),
	})
	s, err := token.SignedString([]byte("test-key"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// TODO(hasheddan): these tests are testing through to the underlying config
// package more than we would like. We should consider refactoring the config
// package to make it more mockable.

var _ credentials.Helper = &Helper{}

func TestGet(t *testing.T) {
	testServer := "xpkg.upbound.io"
	testProfile := "test"
	validToken := testToken(t, time.Now().Add(1*time.Hour))
	expiredToken := testToken(t, time.Now().Add(-1*time.Hour))
	errBoom := errors.New("boom")
	type args struct {
		server string
	}
	type want struct {
		user   string
		secret string
		err    error
	}
	cases := map[string]struct {
		reason string
		args   args
		opts   []Opt
		want   want
	}{
		"ErrorUnsupportedDomain": {
			reason: "Should return error if domain is not supported.",
			args: args{
				server: testServer,
			},
			opts: []Opt{
				WithDomain("registry.upbound.io"),
			},
			want: want{
				err: errors.New(errUnsupportedDomain),
			},
		},
		"ErrorInitializeSource": {
			reason: "Should return error if we fail to initialize source.",
			args: args{
				server: testServer,
			},
			opts: []Opt{
				WithSource(&config.MockSource{
					InitializeFn: func() error {
						return errBoom
					},
				}),
			},
			want: want{
				err: errors.Wrap(errBoom, errInitializeSource),
			},
		},
		"ErrorExtractConfig": {
			reason: "Should return error if we fail to extract config.",
			args: args{
				server: testServer,
			},
			opts: []Opt{
				WithSource(&config.MockSource{
					InitializeFn: func() error {
						return nil
					},
					GetConfigFn: func() (*config.Config, error) {
						return nil, errBoom
					},
				}),
			},
			want: want{
				err: errors.Wrap(errBoom, errExtractConfig),
			},
		},
		"ErrorGetDefault": {
			reason: "If no profile is specified and we fail to get default return error.",
			args: args{
				server: testServer,
			},
			opts: []Opt{
				WithSource(&config.MockSource{
					InitializeFn: func() error {
						return nil
					},
					GetConfigFn: func() (*config.Config, error) {
						return &config.Config{}, nil
					},
				}),
			},
			want: want{
				err: errors.Wrap(errors.New("no default profile specified"), errGetDefaultProfile),
			},
		},
		"ErrorGetProfile": {
			reason: "If we fail to get the specified profile return error.",
			args: args{
				server: testServer,
			},
			opts: []Opt{
				WithProfile(testProfile),
				WithSource(&config.MockSource{
					InitializeFn: func() error {
						return nil
					},
					GetConfigFn: func() (*config.Config, error) {
						return &config.Config{}, nil
					},
				}),
			},
			want: want{
				err: errors.Wrap(errors.Errorf("profile not found with identifier: %s", testProfile), errGetProfile),
			},
		},
		"ErrorEmptySession": {
			reason: "Should return credentials not found if session is empty.",
			args: args{
				server: testServer,
			},
			opts: []Opt{
				WithProfile(testProfile),
				WithSource(&config.MockSource{
					InitializeFn: func() error {
						return nil
					},
					GetConfigFn: func() (*config.Config, error) {
						return &config.Config{
							Upbound: config.Upbound{
								Profiles: map[string]profile.Profile{
									testProfile: {
										Session: "",
									},
								},
							},
						}, nil
					},
				}),
			},
			want: want{
				err: credentials.NewErrCredentialsNotFound(),
			},
		},
		"ErrorExpiredSession": {
			reason: "Should return credentials not found if session token is expired.",
			args: args{
				server: testServer,
			},
			opts: []Opt{
				WithProfile(testProfile),
				WithSource(&config.MockSource{
					InitializeFn: func() error {
						return nil
					},
					GetConfigFn: func() (*config.Config, error) {
						return &config.Config{
							Upbound: config.Upbound{
								Profiles: map[string]profile.Profile{
									testProfile: {
										Session: expiredToken,
									},
								},
							},
						}, nil
					},
				}),
			},
			want: want{
				err: credentials.NewErrCredentialsNotFound(),
			},
		},
		"Success": {
			reason: "If we successfully get profile with a valid session return credentials.",
			args: args{
				server: testServer,
			},
			opts: []Opt{
				WithProfile(testProfile),
				WithSource(&config.MockSource{
					InitializeFn: func() error {
						return nil
					},
					GetConfigFn: func() (*config.Config, error) {
						return &config.Config{
							Upbound: config.Upbound{
								Profiles: map[string]profile.Profile{
									testProfile: {
										Session: validToken,
									},
								},
							},
						}, nil
					},
				}),
			},
			want: want{
				user:   defaultDockerUser,
				secret: validToken,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			user, secret, err := New(tc.opts...).Get(tc.args.server)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGet(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.user, user); diff != "" {
				t.Errorf("\n%s\nGet(...): -want user, +got user:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.secret, secret); diff != "" {
				t.Errorf("\n%s\nGet(...): -want secret, +got secret:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestAdd(t *testing.T) {
	err := New().Add(nil)
	if diff := cmp.Diff(errors.New(errUnimplemented), err, test.EquateErrors()); diff != "" {
		t.Errorf("\nAdd(...): -want error, +got error:\n%s", diff)
	}
}

func TestDelete(t *testing.T) {
	err := New().Delete("")
	if diff := cmp.Diff(errors.New(errUnimplemented), err, test.EquateErrors()); diff != "" {
		t.Errorf("\nDelete(...): -want error, +got error:\n%s", diff)
	}
}

func TestList(t *testing.T) {
	_, err := New().List()
	if diff := cmp.Diff(errors.New(errUnimplemented), err, test.EquateErrors()); diff != "" {
		t.Errorf("\nList(...): -want error, +got error:\n%s", diff)
	}
}
