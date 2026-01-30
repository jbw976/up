// Copyright 2025 Upbound Inc.
// All rights reserved

package supportbundle

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func TestMatchesPattern(t *testing.T) {
	tests := map[string]struct {
		namespace string
		pattern   string
		want      bool
	}{
		"exact match": {
			namespace: "upbound-system",
			pattern:   "upbound-system",
			want:      true,
		},
		"exact match no match": {
			namespace: "upbound-system",
			pattern:   "crossplane-system",
			want:      false,
		},
		"glob prefix match": {
			namespace: "upbound-system",
			pattern:   "upbound-*",
			want:      true,
		},
		"glob prefix no match": {
			namespace: "crossplane-system",
			pattern:   "upbound-*",
			want:      false,
		},
		"glob suffix match": {
			namespace: "upbound-system",
			pattern:   "*-system",
			want:      true,
		},
		"glob suffix no match": {
			namespace: "upbound-test",
			pattern:   "*-system",
			want:      false,
		},
		"glob middle match": {
			namespace: "upbound-test-system",
			pattern:   "upbound-*-system",
			want:      true,
		},
		"glob multiple wildcards": {
			namespace: "upbound-test-namespace",
			pattern:   "upbound-*-*",
			want:      true,
		},
		"glob question mark match": {
			namespace: "upbound-test",
			pattern:   "upbound-????",
			want:      true,
		},
		"glob question mark no match": {
			namespace: "upbound-testing",
			pattern:   "upbound-????",
			want:      false,
		},
		"no glob characters": {
			namespace: "upbound-system",
			pattern:   "upbound",
			want:      false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := matchesPattern(tt.namespace, tt.pattern)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("matchesPattern(%q, %q): -want, +got\n%s", tt.namespace, tt.pattern, diff)
			}
		})
	}
}

func TestMatchNamespaces(t *testing.T) {
	namespaces := []corev1.Namespace{
		{ObjectMeta: metav1.ObjectMeta{Name: "upbound-system"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "upbound-test"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "crossplane-system"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
	}

	tests := map[string]struct {
		patterns []string
		want     []string
	}{
		"exact match single": {
			patterns: []string{"upbound-system"},
			want:     []string{"upbound-system"},
		},
		"glob prefix single": {
			patterns: []string{"upbound-*"},
			want:     []string{"upbound-system", "upbound-test"},
		},
		"glob suffix": {
			patterns: []string{"*-system"},
			want:     []string{"upbound-system", "crossplane-system", "kube-system"},
		},
		"multiple exact patterns": {
			patterns: []string{"upbound-system", "crossplane-system"},
			want:     []string{"upbound-system", "crossplane-system"},
		},
		"multiple glob patterns": {
			patterns: []string{"upbound-*", "kube-*"},
			want:     []string{"upbound-system", "upbound-test", "kube-system"},
		},
		"mixed exact and glob": {
			patterns: []string{"default", "upbound-*"},
			want:     []string{"default", "upbound-system", "upbound-test"},
		},
		"no matches": {
			patterns: []string{"nonexistent-*"},
			want:     []string{},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := matchNamespaces(namespaces, tt.patterns)
			if diff := cmp.Diff(tt.want, got, cmpopts.SortSlices(func(a, b string) bool {
				return a < b
			})); diff != "" {
				t.Errorf("%s\nmatchNamespaces(...): -want, +got\n%s", name, diff)
			}
		})
	}
}

func TestShouldExcludeNamespace(t *testing.T) {
	tests := map[string]struct {
		namespace       string
		excludePatterns []string
		want            bool
	}{
		"exact match exclude": {
			namespace:       "upbound-system",
			excludePatterns: []string{"upbound-system"},
			want:            true,
		},
		"exact match no exclude": {
			namespace:       "upbound-system",
			excludePatterns: []string{"crossplane-system"},
			want:            false,
		},
		"glob prefix exclude": {
			namespace:       "upbound-test",
			excludePatterns: []string{"upbound-*"},
			want:            true,
		},
		"glob prefix no exclude": {
			namespace:       "crossplane-system",
			excludePatterns: []string{"upbound-*"},
			want:            false,
		},
		"multiple patterns match": {
			namespace:       "upbound-test",
			excludePatterns: []string{"kube-*", "upbound-*"},
			want:            true,
		},
		"multiple patterns no match": {
			namespace:       "default",
			excludePatterns: []string{"kube-*", "upbound-*"},
			want:            false,
		},
		"empty patterns": {
			namespace:       "upbound-system",
			excludePatterns: []string{},
			want:            false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := shouldExcludeNamespace(tt.namespace, tt.excludePatterns)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("%s\nshouldExcludeNamespace(%q, %v): -want, +got\n%s", name, tt.namespace, tt.excludePatterns, diff)
			}
		})
	}
}

func TestDetermineNamespaces(t *testing.T) {
	ctx := context.Background()

	type args struct {
		includeNamespaces []string
		excludeNamespaces []string
	}

	cases := map[string]struct {
		reason string
		args   args
		setup  func(*fake.Clientset)
		want   []string
	}{
		"DefaultWithNilClient": {
			reason: "Should return default namespaces when client is nil.",
			setup:  nil,
			want:   []string{"crossplane-system", "upbound-system"},
		},
		"DefaultWithEmptyClient": {
			reason: "Should return default namespaces when client has no labeled namespaces.",
			setup:  func(_ *fake.Clientset) {},
			want:   []string{"crossplane-system", "upbound-system"},
		},
		"DefaultIncludesControlplaneNameLabeledNamespace": {
			reason: "Should include namespaces labeled with internal.spaces.upbound.io/controlplane-name.",
			setup: func(client *fake.Clientset) {
				_, _ = client.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "space-cp-1",
						Labels: map[string]string{"internal.spaces.upbound.io/controlplane-name": "cp-1"},
					},
				}, metav1.CreateOptions{})
			},
			want: []string{"crossplane-system", "space-cp-1", "upbound-system"},
		},
		"DefaultIncludesSpacesGroupLabeledNamespace": {
			reason: "Should include namespaces labeled with spaces.upbound.io/group.",
			setup: func(client *fake.Clientset) {
				_, _ = client.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "space-group-1",
						Labels: map[string]string{"spaces.upbound.io/group": "group-1"},
					},
				}, metav1.CreateOptions{})
			},
			want: []string{"crossplane-system", "space-group-1", "upbound-system"},
		},
		"DefaultDeduplicatesNamespaceWithBothLabels": {
			reason: "Should include namespace with both labels only once.",
			setup: func(client *fake.Clientset) {
				_, _ = client.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "space-both",
						Labels: map[string]string{
							"internal.spaces.upbound.io/controlplane-name": "cp-1",
							"spaces.upbound.io/group":                      "group-1",
						},
					},
				}, metav1.CreateOptions{})
			},
			want: []string{"crossplane-system", "space-both", "upbound-system"},
		},
		"ExcludePatternFiltersDefaultNamespaces": {
			reason: "Should filter out namespaces matching exclude pattern.",
			args: args{
				excludeNamespaces: []string{"crossplane-system"},
			},
			setup: func(_ *fake.Clientset) {},
			want:  []string{"upbound-system"},
		},
		"IncludePatternWithClient": {
			reason: "Should resolve include patterns against cluster namespaces.",
			args: args{
				includeNamespaces: []string{"upbound-*"},
			},
			setup: func(client *fake.Clientset) {
				for _, name := range []string{"upbound-system", "other-ns"} {
					_, _ = client.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
						ObjectMeta: metav1.ObjectMeta{Name: name},
					}, metav1.CreateOptions{})
				}
			},
			want: []string{"upbound-system"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var kubeClient kubernetes.Interface
			if tc.setup != nil {
				client := fake.NewClientset()
				tc.setup(client)
				kubeClient = client
			}

			got := determineNamespaces(ctx, kubeClient, tc.args.includeNamespaces, tc.args.excludeNamespaces)

			if diff := cmp.Diff(tc.want, got, cmpopts.SortSlices(func(a, b string) bool {
				return a < b
			})); diff != "" {
				t.Errorf("\n%s\ndetermineNamespaces(): -want, +got\n%s", tc.reason, diff)
			}
		})
	}
}

func TestLoadConfigFromFile(t *testing.T) {
	type result struct {
		Spec     bool
		Redactor bool
		Err      error
	}

	tests := map[string]struct {
		setupFS func() afero.Fs
		want    result
	}{
		"valid SupportBundle config without redactor": {
			setupFS: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "/test-config.yaml", []byte(`apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: support-bundle
spec:
  collectors:
    - clusterInfo: {}
    - clusterResources:
        namespaces:
          - crossplane-system
          - upbound-system
`), 0o644)
				return fs
			},
			want: result{
				Spec:     true,
				Redactor: false,
				Err:      nil,
			},
		},
		"valid SupportBundle config with redactor": {
			setupFS: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "/test-config.yaml", []byte(`apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: support-bundle
spec:
  collectors:
    - clusterInfo: {}
---
apiVersion: troubleshoot.sh/v1beta2
kind: Redactor
metadata:
  name: custom-redactors
spec:
  redactors:
    - name: custom-redactor
      removals:
        regex:
          - redactor: ".*password.*"
`), 0o644)
				return fs
			},
			want: result{
				Spec:     true,
				Redactor: true,
				Err:      nil,
			},
		},
		"file not found": {
			setupFS: afero.NewMemMapFs,
			want: result{
				Spec:     false,
				Redactor: false,
				Err:      cmpopts.AnyError,
			},
		},
		"invalid YAML": {
			setupFS: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "/test-config.yaml", []byte(`invalid: yaml: content
  - broken
`), 0o644)
				return fs
			},
			want: result{
				Spec:     false,
				Redactor: false,
				Err:      cmpopts.AnyError,
			},
		},
		"multiple redactors should error": {
			setupFS: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "/test-config.yaml", []byte(`apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: support-bundle
spec:
  collectors:
    - clusterInfo: {}
---
apiVersion: troubleshoot.sh/v1beta2
kind: Redactor
metadata:
  name: redactor-1
spec:
  redactors:
    - name: redactor-1
---
apiVersion: troubleshoot.sh/v1beta2
kind: Redactor
metadata:
  name: redactor-2
spec:
  redactors:
    - name: redactor-2
`), 0o644)
				return fs
			},
			want: result{
				Spec:     false,
				Redactor: false,
				Err:      cmpopts.AnyError,
			},
		},
		"non-redactor document after SupportBundle": {
			setupFS: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "/test-config.yaml", []byte(`apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: support-bundle
spec:
  collectors:
    - clusterInfo: {}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
`), 0o644)
				return fs
			},
			want: result{
				Spec:     false,
				Redactor: false,
				Err:      cmpopts.AnyError,
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			fs := tt.setupFS()
			configPath := "/test-config.yaml"

			cmd := &collectCmd{
				fs: fs,
			}

			spec, redactor, err := cmd.loadConfigFromFile(configPath)

			got := result{
				Spec:     spec != nil,
				Redactor: redactor != nil,
				Err:      err,
			}

			if diff := cmp.Diff(tt.want, got, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("loadConfigFromFile() -want, +got\n%s", diff)
			}
		})
	}
}
