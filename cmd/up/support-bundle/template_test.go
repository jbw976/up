// Copyright 2025 Upbound Inc.
// All rights reserved

package supportbundle

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	"github.com/upbound/up/internal/supportbundle/defaults"
)

func TestTemplateCmd_Run(t *testing.T) {
	type args struct {
		cmd *templateCmd
	}
	type want struct {
		err              error
		expectedTemplate func() (troubleshootv1beta2.SupportBundle, troubleshootv1beta2.Redactor)
	}

	cases := map[string]struct {
		reason string
		args   args
		setup  func(*fake.Clientset)
		want   want
	}{
		"SuccessWithKubeconfigSet": {
			reason: "Should successfully generate template",
			args: args{
				cmd: &templateCmd{},
			},
			setup: func(_ *fake.Clientset) {},
			want: want{
				err: nil,
				expectedTemplate: func() (troubleshootv1beta2.SupportBundle, troubleshootv1beta2.Redactor) {
					return defaultExpectedTemplate([]string{"crossplane-system", "upbound-system"})
				},
			},
		},
		"WithIncludeNamespaces": {
			reason: "Should successfully generate template with include namespaces flag",
			args: args{
				cmd: &templateCmd{
					commonFlags: commonFlags{
						IncludeNamespaces: []string{"test-ns"},
					},
				},
			},
			setup: func(client *fake.Clientset) {
				client.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{Name: "test-ns"},
				}, metav1.CreateOptions{})
				client.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{Name: "other-ns"},
				}, metav1.CreateOptions{})
			},
			want: want{
				err: nil,
				expectedTemplate: func() (troubleshootv1beta2.SupportBundle, troubleshootv1beta2.Redactor) {
					return defaultExpectedTemplate([]string{"test-ns"})
				},
			},
		},
		"WithExcludeNamespaces": {
			reason: "Should successfully generate template with exclude namespaces flag",
			args: args{
				cmd: &templateCmd{
					commonFlags: commonFlags{
						ExcludeNamespaces: []string{"crossplane-system"},
					},
				},
			},
			setup: func(_ *fake.Clientset) {},
			want: want{
				err: nil,
				expectedTemplate: func() (troubleshootv1beta2.SupportBundle, troubleshootv1beta2.Redactor) {
					return defaultExpectedTemplate([]string{"upbound-system"})
				},
			},
		},
		"WithoutKubeconfig": {
			reason: "Should fall back to default namespaces when kClient is nil",
			args: args{
				cmd: &templateCmd{
					kClient: nil,
				},
			},
			setup: nil,
			want: want{
				err: nil,
				expectedTemplate: func() (troubleshootv1beta2.SupportBundle, troubleshootv1beta2.Redactor) {
					return defaultExpectedTemplate([]string{"crossplane-system", "upbound-system"})
				},
			},
		},
		"WithIncludeNamespacesButNoClient": {
			reason: "Should use include namespaces directly when client is nil (patterns not resolved)",
			args: args{
				cmd: &templateCmd{
					commonFlags: commonFlags{
						IncludeNamespaces: []string{"test-ns"},
					},
					kClient: nil,
				},
			},
			setup: nil,
			want: want{
				err: nil,
				expectedTemplate: func() (troubleshootv1beta2.SupportBundle, troubleshootv1beta2.Redactor) {
					return defaultExpectedTemplate([]string{"test-ns"})
				},
			},
		},
		"WithExcludeNamespacesButNoClient": {
			reason: "Should apply exclude filter to default namespaces when client is nil",
			args: args{
				cmd: &templateCmd{
					commonFlags: commonFlags{
						ExcludeNamespaces: []string{"crossplane-system"},
					},
					kClient: nil,
				},
			},
			setup: nil,
			want: want{
				err: nil,
				expectedTemplate: func() (troubleshootv1beta2.SupportBundle, troubleshootv1beta2.Redactor) {
					return defaultExpectedTemplate([]string{"upbound-system"})
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()

			cmdBuf := &bytes.Buffer{}
			tc.args.cmd.out = cmdBuf

			if tc.args.cmd.kClient == nil && tc.setup != nil {
				client := fake.NewSimpleClientset()
				tc.setup(client)
				tc.args.cmd.kClient = client
			}

			err := tc.args.cmd.Run(ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\n-want error, +got error:\n%s", tc.reason, diff)
			}

			output := cmdBuf.String()
			parts := strings.Split(output, "---")
			if len(parts) != 2 {
				t.Errorf("\n%s\nexpected 2 parts separated by '---', got %d", tc.reason, len(parts))
				return
			}

			var gotSupportBundle troubleshootv1beta2.SupportBundle
			if err := yaml.Unmarshal([]byte(parts[0]), &gotSupportBundle); err != nil {
				t.Errorf("\n%s\nfailed to parse SupportBundle YAML: %v", tc.reason, err)
				return
			}

			var gotRedactor troubleshootv1beta2.Redactor
			if err := yaml.Unmarshal([]byte(parts[1]), &gotRedactor); err != nil {
				t.Errorf("\n%s\nfailed to parse Redactor YAML: %v", tc.reason, err)
				return
			}

			expectedSupportBundle, expectedRedactor := tc.want.expectedTemplate()

			opts := []cmp.Option{
				cmpopts.EquateEmpty(),
			}

			if diff := cmp.Diff(expectedSupportBundle, gotSupportBundle, opts...); diff != "" {
				t.Errorf("\n%s\n-want SupportBundle, +got SupportBundle:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(expectedRedactor, gotRedactor, opts...); diff != "" {
				t.Errorf("\n%s\n-want Redactor, +got Redactor:\n%s", tc.reason, diff)
			}
		})
	}
}

// defaultExpectedTemplate returns the default expected template structure.
// This is used as a base that test cases can modify.
func defaultExpectedTemplate(expectedNamespaces []string) (troubleshootv1beta2.SupportBundle, troubleshootv1beta2.Redactor) {
	spec := defaults.SupportBundleSpec(expectedNamespaces, true)

	supportBundle := troubleshootv1beta2.SupportBundle{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "troubleshoot.sh/v1beta2",
			Kind:       "SupportBundle",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "support-bundle",
		},
		Spec: *spec,
	}

	defaultRedactor := defaults.Redactors()
	redactor := troubleshootv1beta2.Redactor{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "troubleshoot.sh/v1beta2",
			Kind:       "Redactor",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "default-redactors",
		},
		Spec: defaultRedactor.Spec,
	}

	return supportBundle, redactor
}
