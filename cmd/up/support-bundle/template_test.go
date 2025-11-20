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
	"sigs.k8s.io/yaml"
)

func TestTemplateCmd_Run(t *testing.T) {
	type args struct {
		cmd *templateCmd
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
			reason: "Should successfully generate template",
			args: args{
				cmd: &templateCmd{
					out: &bytes.Buffer{},
				},
			},
			want: want{
				err: nil,
			},
		},
		"WithIncludeNamespaces": {
			reason: "Should successfully generate template with include namespaces flag",
			args: args{
				cmd: &templateCmd{
					commonFlags: commonFlags{
						IncludeNamespaces: []string{"test-ns"},
					},
					out: &bytes.Buffer{},
				},
			},
			want: want{
				err: nil,
			},
		},
		"WithExcludeNamespaces": {
			reason: "Should successfully generate template with exclude namespaces flag",
			args: args{
				cmd: &templateCmd{
					commonFlags: commonFlags{
						ExcludeNamespaces: []string{"kube-system"},
					},
					out: &bytes.Buffer{},
				},
			},
			want: want{
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			err := tc.args.cmd.Run(ctx)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\n-want error, +got error:\n%s", tc.reason, diff)
			}

			cmdBuf, ok := tc.args.cmd.out.(*bytes.Buffer)
			if !ok {
				t.Fatal("expected output to be a bytes.Buffer")
			}
			output := cmdBuf.String()

			parts := strings.Split(output, "---")
			if len(parts) != 2 {
				t.Errorf("\n%s\nexpected 2 parts separated by '---', got %d", tc.reason, len(parts))
				return
			}

			var supportBundle troubleshootv1beta2.SupportBundle
			if err := yaml.Unmarshal([]byte(parts[0]), &supportBundle); err != nil {
				t.Errorf("\n%s\nfailed to parse SupportBundle YAML: %v", tc.reason, err)
			}

			var redactor troubleshootv1beta2.Redactor
			if err := yaml.Unmarshal([]byte(parts[1]), &redactor); err != nil {
				t.Errorf("\n%s\nfailed to parse Redactor YAML: %v", tc.reason, err)
			}
		})
	}
}
