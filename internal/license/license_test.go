// Copyright 2025 Upbound Inc.
// All rights reserved

package license

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/upbound/controller-manager/apis/licensing/v1alpha1"
)

func TestFromUXPv2(t *testing.T) {
	type args struct {
		cl client.Client
	}
	type want struct {
		license *v1alpha1.License
		err     error
	}

	testCases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Success": {
			reason: "Should return license when found in cluster.",
			args: args{
				cl: fake.NewClientBuilder().
					WithScheme(func() *runtime.Scheme {
						s := runtime.NewScheme()
						v1alpha1.AddToScheme(s)
						return s
					}()).
					WithObjects(&v1alpha1.License{
						ObjectMeta: metav1.ObjectMeta{
							Name: v1alpha1.LicenseName,
						},
						Spec: v1alpha1.LicenseSpec{
							SecretRef: &v1alpha1.LicenseSecretRef{
								Name:      "test-secret",
								Namespace: "test-namespace",
								Key:       "license",
							},
						},
					}).
					Build(),
			},
			want: want{
				license: &v1alpha1.License{
					ObjectMeta: metav1.ObjectMeta{
						Name: v1alpha1.LicenseName,
					},
					Spec: v1alpha1.LicenseSpec{
						SecretRef: &v1alpha1.LicenseSecretRef{
							Name:      "test-secret",
							Namespace: "test-namespace",
							Key:       "license",
						},
					},
				},
			},
		},
		"LicenseNotFound": {
			reason: "Should return ErrLicenseNotFound when license resource is not found.",
			args: args{
				cl: fake.NewClientBuilder().
					WithScheme(func() *runtime.Scheme {
						s := runtime.NewScheme()
						v1alpha1.AddToScheme(s)
						return s
					}()).
					Build(),
			},
			want: want{
				license: nil,
				err:     ErrLicenseNotFound,
			},
		},
		"GetLicenseError": {
			reason: "Should return wrapped error when client Get fails with non-NotFound error.",
			args: args{
				cl: &test.MockClient{
					MockGet: test.NewMockGetFn(errors.New("client error")),
					MockScheme: test.NewMockSchemeFn(func() *runtime.Scheme {
						s := runtime.NewScheme()
						v1alpha1.AddToScheme(s)
						return s
					}()),
				},
			},
			want: want{
				license: nil,
				err:     errors.Wrap(errors.New("client error"), "failed to get license \"uxp\""),
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := FromUXPv2(t.Context(), tc.args.cl)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("%s\nFromUXPv2(...) error -want +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.license, got, cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion")); diff != "" {
				t.Errorf("%s\nFromUXPv2(...) -want +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestFileFromUXPv2(t *testing.T) {
	type args struct {
		cl client.Client
	}
	type want struct {
		file []byte
		err  error
	}

	testCases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Success": {
			reason: "Should return license file when license and secret exist.",
			args: args{
				cl: fake.NewClientBuilder().
					WithScheme(func() *runtime.Scheme {
						s := runtime.NewScheme()
						v1alpha1.AddToScheme(s)
						corev1.AddToScheme(s)
						return s
					}()).
					WithObjects(
						&v1alpha1.License{
							ObjectMeta: metav1.ObjectMeta{
								Name: v1alpha1.LicenseName,
							},
							Spec: v1alpha1.LicenseSpec{
								SecretRef: &v1alpha1.LicenseSecretRef{
									Name:      "test-secret",
									Namespace: "test-namespace",
									Key:       "license",
								},
							},
						},
						&corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "test-secret",
								Namespace: "test-namespace",
							},
							Data: map[string][]byte{
								"license": []byte("license-content"),
							},
						},
					).
					Build(),
			},
			want: want{
				file: []byte("license-content"),
			},
		},
		"CommunityLicense": {
			reason: "Should return ErrCommunity when license has no SecretRef.",
			args: args{
				cl: fake.NewClientBuilder().
					WithScheme(func() *runtime.Scheme {
						s := runtime.NewScheme()
						v1alpha1.AddToScheme(s)
						return s
					}()).
					WithObjects(&v1alpha1.License{
						ObjectMeta: metav1.ObjectMeta{
							Name: v1alpha1.LicenseName,
						},
						Spec: v1alpha1.LicenseSpec{
							SecretRef: nil,
						},
					}).
					Build(),
			},
			want: want{
				file: nil,
				err:  ErrCommunity,
			},
		},
		"LicenseNotFound": {
			reason: "Should return ErrLicenseNotFound when license resource is not found.",
			args: args{
				cl: fake.NewClientBuilder().
					WithScheme(func() *runtime.Scheme {
						s := runtime.NewScheme()
						v1alpha1.AddToScheme(s)
						return s
					}()).
					Build(),
			},
			want: want{
				file: nil,
				err:  ErrLicenseNotFound,
			},
		},
		"SecretNotFound": {
			reason: "Should return wrapped error when secret is not found.",
			args: args{
				cl: fake.NewClientBuilder().
					WithScheme(func() *runtime.Scheme {
						s := runtime.NewScheme()
						v1alpha1.AddToScheme(s)
						corev1.AddToScheme(s)
						return s
					}()).
					WithObjects(&v1alpha1.License{
						ObjectMeta: metav1.ObjectMeta{
							Name: v1alpha1.LicenseName,
						},
						Spec: v1alpha1.LicenseSpec{
							SecretRef: &v1alpha1.LicenseSecretRef{
								Name:      "test-secret",
								Namespace: "test-namespace",
								Key:       "license",
							},
						},
					}).
					Build(),
			},
			want: want{
				file: nil,
				err:  errors.Wrap(kerrors.NewNotFound(schema.GroupResource{Group: "", Resource: "secrets"}, "test-secret"), "failed to get license secret \"test-namespace/test-secret\""),
			},
		},
		"SecretMissingKey": {
			reason: "Should return error when secret is missing the required key.",
			args: args{
				cl: fake.NewClientBuilder().
					WithScheme(func() *runtime.Scheme {
						s := runtime.NewScheme()
						v1alpha1.AddToScheme(s)
						corev1.AddToScheme(s)
						return s
					}()).
					WithObjects(
						&v1alpha1.License{
							ObjectMeta: metav1.ObjectMeta{
								Name: v1alpha1.LicenseName,
							},
							Spec: v1alpha1.LicenseSpec{
								SecretRef: &v1alpha1.LicenseSecretRef{
									Name:      "test-secret",
									Namespace: "test-namespace",
									Key:       "license",
								},
							},
						},
						&corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "test-secret",
								Namespace: "test-namespace",
							},
							Data: map[string][]byte{
								"other-key": []byte("other-content"),
							},
						},
					).
					Build(),
			},
			want: want{
				file: nil,
				err:  errors.New("license secret \"test-namespace/test-secret\" is missing key: license"),
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := FileFromUXPv2(t.Context(), tc.args.cl)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("%s\nFileFromUXPv2(...) error -want +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.file, got); diff != "" {
				t.Errorf("%s\nFileFromUXPv2(...) -want +got:\n%s", tc.reason, diff)
			}
		})
	}
}
