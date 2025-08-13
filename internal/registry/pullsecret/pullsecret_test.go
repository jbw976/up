// Copyright 2025 Upbound Inc.
// All rights reserved

package pullsecret

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
)

func TestManager_CreateOrUpdate(t *testing.T) {
	type args struct {
		name      string
		namespace string
		username  string
		password  string
		endpoint  string
	}

	type want struct {
		err    error
		secret *corev1.Secret
	}

	cases := map[string]struct {
		reason string
		args   args
		setup  func(*fake.Clientset)
		want   want
	}{
		"SuccessCreateBoth": {
			reason: "Should successfully create both namespace and secret when neither exists.",
			args: args{
				name:      "test-pull-secret",
				namespace: "test-namespace",
				username:  "testuser",
				password:  "testpass",
				endpoint:  "registry.example.com",
			},
			want: want{
				secret: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pull-secret",
						Namespace: "test-namespace",
					},
					Type: corev1.SecretTypeDockerConfigJson,
					Data: map[string][]byte{
						corev1.DockerConfigJsonKey: []byte(`{"auths":{"registry.example.com":{"username":"testuser","password":"testpass","auth":"dGVzdHVzZXI6dGVzdHBhc3M="}}}`),
					},
				},
			},
		},
		"SuccessNamespaceExists": {
			reason: "Should successfully create secret when namespace already exists.",
			args: args{
				name:      "existing-secret",
				namespace: "existing-namespace",
				username:  "user",
				password:  "pass",
				endpoint:  "my-registry.io",
			},
			setup: func(client *fake.Clientset) {
				// Pre-create the namespace
				client.CoreV1().Namespaces().Create(t.Context(), &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "existing-namespace",
					},
				}, metav1.CreateOptions{})
			},
			want: want{
				secret: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "existing-secret",
						Namespace: "existing-namespace",
					},
					Type: corev1.SecretTypeDockerConfigJson,
					Data: map[string][]byte{
						corev1.DockerConfigJsonKey: []byte(`{"auths":{"my-registry.io":{"username":"user","password":"pass","auth":"dXNlcjpwYXNz"}}}`),
					},
				},
			},
		},
		"SuccessUpdateExistingSecret": {
			reason: "Should successfully update existing secret with new credentials.",
			args: args{
				name:      "update-secret",
				namespace: "update-namespace",
				username:  "newuser",
				password:  "newpass",
				endpoint:  "updated-registry.com",
			},
			setup: func(client *fake.Clientset) {
				// Pre-create namespace and old secret
				client.CoreV1().Namespaces().Create(t.Context(), &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "update-namespace",
					},
				}, metav1.CreateOptions{})

				client.CoreV1().Secrets("update-namespace").Create(t.Context(), &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "update-secret",
						Namespace: "update-namespace",
					},
					Type: corev1.SecretTypeDockerConfigJson,
					Data: map[string][]byte{
						corev1.DockerConfigJsonKey: []byte(`{"auths":{"old-registry.com":{"username":"olduser","password":"oldpass","auth":"b2xkdXNlcjpvbGRwYXNz"}}}`),
					},
				}, metav1.CreateOptions{})
			},
			want: want{
				secret: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "update-secret",
						Namespace: "update-namespace",
					},
					Type: corev1.SecretTypeDockerConfigJson,
					Data: map[string][]byte{
						corev1.DockerConfigJsonKey: []byte(`{"auths":{"updated-registry.com":{"username":"newuser","password":"newpass","auth":"bmV3dXNlcjpuZXdwYXNz"}}}`),
					},
				},
			},
		},
		"ErrorNamespaceCreateFailure": {
			reason: "Should return error when namespace creation fails with non-AlreadyExists error.",
			args: args{
				name:      "fail-secret",
				namespace: "fail-namespace",
				username:  "user",
				password:  "pass",
				endpoint:  "registry.fail",
			},
			setup: func(client *fake.Clientset) {
				// Simulate namespace creation failure
				client.PrependReactor("create", "namespaces", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
					if action.GetResource().Resource == "namespaces" {
						createAction := action.(ktesting.CreateAction)
						if createAction.GetObject().(*corev1.Namespace).Name == "fail-namespace" {
							return true, nil, kerrors.NewForbidden(schema.GroupResource{Group: "", Resource: "namespaces"}, "fail-namespace", nil)
						}
					}
					return false, nil, nil
				})
			},
			want: want{
				err: errors.Wrap(kerrors.NewForbidden(schema.GroupResource{Group: "", Resource: "namespaces"}, "fail-namespace", nil), "failed to create pull secret namespace \"fail-namespace\""),
			},
		},
		"SuccessEmptyCredentials": {
			reason: "Should successfully create secret with empty credentials.",
			args: args{
				name:      "empty-creds",
				namespace: "empty-namespace",
				username:  "",
				password:  "",
				endpoint:  "empty-registry.com",
			},
			want: want{
				secret: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "empty-creds",
						Namespace: "empty-namespace",
					},
					Type: corev1.SecretTypeDockerConfigJson,
					Data: map[string][]byte{
						corev1.DockerConfigJsonKey: []byte(`{"auths":{"empty-registry.com":{"auth":"Og=="}}}`),
					},
				},
			},
		},
		"SuccessSpecialCharacters": {
			reason: "Should successfully handle special characters in credentials and endpoint.",
			args: args{
				name:      "special-secret",
				namespace: "special-namespace",
				username:  "user@domain.com",
				password:  "p@ssw0rd!#$",
				endpoint:  "my-registry.example.com:8080",
			},
			want: want{
				secret: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "special-secret",
						Namespace: "special-namespace",
					},
					Type: corev1.SecretTypeDockerConfigJson,
					Data: map[string][]byte{
						corev1.DockerConfigJsonKey: []byte(`{"auths":{"my-registry.example.com:8080":{"username":"user@domain.com","password":"p@ssw0rd!#$","auth":"dXNlckBkb21haW4uY29tOnBAc3N3MHJkISMk"}}}`),
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			client := fake.NewSimpleClientset()

			if tc.setup != nil {
				tc.setup(client)
			}

			mgr := NewManager(
				client,
				tc.args.name,
				tc.args.namespace,
				tc.args.username,
				tc.args.password,
				tc.args.endpoint,
			)

			err := mgr.CreateOrUpdate(t.Context())

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nCreateOrUpdate(...): -want error, +got error:\n%s", tc.reason, diff)
				return
			}

			if tc.want.secret != nil {
				gotSecret, err := client.CoreV1().Secrets(tc.want.secret.Namespace).Get(t.Context(), tc.want.secret.Name, metav1.GetOptions{})
				if err != nil {
					t.Errorf("\n%s\nFailed to get secret %s/%s: %v", tc.reason, tc.want.secret.Namespace, tc.want.secret.Name, err)
					return
				}

				if diff := cmp.Diff(tc.want.secret, gotSecret, cmpopts.IgnoreFields(metav1.ObjectMeta{},
					"ResourceVersion", "UID", "CreationTimestamp", "ManagedFields")); diff != "" {
					t.Errorf("\n%s\nCreateOrUpdate(...): -want secret, +got secret:\n%s", tc.reason, diff)
				}
			}
		})
	}
}
