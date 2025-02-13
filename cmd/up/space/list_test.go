// Copyright 2025 Upbound Inc.
// All rights reserved

package space

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/upbound/up-sdk-go"
	upboundv1alpha1 "github.com/upbound/up-sdk-go/apis/upbound/v1alpha1"
	uerrors "github.com/upbound/up-sdk-go/errors"
	"github.com/upbound/up-sdk-go/fake"
	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

func TestListCommand(t *testing.T) {
	acc := "some-account"
	errBoom := errors.New("boom")
	accResp := `{"account": {"name": "some-account", "type": "organization"}, "organization": {"name": "some-account"}}`

	type args struct {
		cmd   *listCmd
		upCtx *upbound.Context
	}
	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NotLoggedIntoCloud": {
			reason: "If the user is not logged into cloud, we expect no error and instead a nice human message.",
			args: args{
				cmd: &listCmd{
					ac: accounts.NewClient(&up.Config{
						Client: &fake.MockClient{
							MockNewRequest: fake.NewMockNewRequestFn(nil, nil),
							MockDo: fake.NewMockDoFn(&uerrors.Error{
								Status: http.StatusUnauthorized,
								Title:  http.StatusText(http.StatusUnauthorized),
							}),
						},
					}),
				},
				upCtx: &upbound.Context{
					Organization: "some-account",
				},
			},
			want: want{
				err: &uerrors.Error{
					Status: http.StatusUnauthorized,
					Title:  http.StatusText(http.StatusUnauthorized),
				},
			},
		},
		"ErrFailedToQueryForAccount": {
			reason: "If the user could not query cloud due to service availability we should return an error.",
			args: args{
				cmd: &listCmd{
					ac: accounts.NewClient(&up.Config{
						Client: &fake.MockClient{
							MockNewRequest: fake.NewMockNewRequestFn(nil, nil),
							MockDo: fake.NewMockDoFn(&uerrors.Error{
								Status: http.StatusServiceUnavailable,
								Title:  http.StatusText(http.StatusServiceUnavailable),
							}),
						},
					}),
				},
				upCtx: &upbound.Context{
					Organization: acc,
				},
			},
			want: want{
				err: errors.Wrap(errors.New(`failed to get Account "some-account": Service Unavailable`), errListSpaces),
			},
		},
		"ErrFailedToListSpaces": {
			reason: "If we received an error from Cloud, we should return an error.",
			args: args{
				cmd: &listCmd{
					ac: accounts.NewClient(&up.Config{
						Client: &fake.MockClient{
							MockNewRequest: fake.NewMockNewRequestFn(nil, nil),
							MockDo: func(_ *http.Request, obj interface{}) error {
								return json.Unmarshal([]byte(accResp), &obj)
							},
						},
					}),
					kc: &test.MockClient{
						MockList: test.NewMockListFn(errBoom),
					},
				},
				upCtx: &upbound.Context{
					Organization: acc,
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errListSpaces),
			},
		},
		"NoSpacesFound": {
			reason: "If we were able to query Cloud, but no spaces were found, we should print a human consumable error.",
			args: args{
				cmd: &listCmd{
					ac: accounts.NewClient(&up.Config{
						Client: &fake.MockClient{
							MockNewRequest: fake.NewMockNewRequestFn(nil, nil),
							MockDo: func(_ *http.Request, obj interface{}) error {
								return json.Unmarshal([]byte(accResp), &obj)
							},
						},
					}),
					kc: &test.MockClient{
						MockList: test.NewMockListFn(nil),
					},
				},
				upCtx: &upbound.Context{
					Organization: acc,
				},
			},
			want: want{},
		},
		"SpacesFound": {
			reason: "If we were able to query Cloud, we should attempt to print what was found.",
			args: args{
				cmd: &listCmd{
					ac: accounts.NewClient(&up.Config{
						Client: &fake.MockClient{
							MockNewRequest: fake.NewMockNewRequestFn(nil, nil),
							MockDo: func(_ *http.Request, obj interface{}) error {
								return json.Unmarshal([]byte(accResp), &obj)
							},
						},
					}),
					kc: &test.MockClient{
						MockList: func(_ context.Context, obj client.ObjectList, _ ...client.ListOption) error {
							list := obj.(*upboundv1alpha1.SpaceList)
							list.Items = []upboundv1alpha1.Space{
								{
									ObjectMeta: metav1.ObjectMeta{
										Name: "some-space",
									},
								},
							}
							return nil
						},
					},
				},
				upCtx: &upbound.Context{
					Organization: acc,
				},
			},
			want: want{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.args.cmd.Run(
				context.Background(),
				upterm.NewNopObjectPrinter(),
				upterm.NewNopTextPrinter(),
				tc.args.upCtx,
			)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nValidateInput(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
