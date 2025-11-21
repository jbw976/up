// Copyright 2025 Upbound Inc.
// All rights reserved

package supportbundle

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMatchesPattern(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		pattern   string
		want      bool
	}{
		{
			name:      "exact match",
			namespace: "upbound-system",
			pattern:   "upbound-system",
			want:      true,
		},
		{
			name:      "exact match no match",
			namespace: "upbound-system",
			pattern:   "crossplane-system",
			want:      false,
		},
		{
			name:      "glob prefix match",
			namespace: "upbound-system",
			pattern:   "upbound-*",
			want:      true,
		},
		{
			name:      "glob prefix no match",
			namespace: "crossplane-system",
			pattern:   "upbound-*",
			want:      false,
		},
		{
			name:      "glob suffix match",
			namespace: "upbound-system",
			pattern:   "*-system",
			want:      true,
		},
		{
			name:      "glob suffix no match",
			namespace: "upbound-test",
			pattern:   "*-system",
			want:      false,
		},
		{
			name:      "glob middle match",
			namespace: "upbound-test-system",
			pattern:   "upbound-*-system",
			want:      true,
		},
		{
			name:      "glob multiple wildcards",
			namespace: "upbound-test-namespace",
			pattern:   "upbound-*-*",
			want:      true,
		},
		{
			name:      "glob question mark match",
			namespace: "upbound-test",
			pattern:   "upbound-????",
			want:      true,
		},
		{
			name:      "glob question mark no match",
			namespace: "upbound-testing",
			pattern:   "upbound-????",
			want:      false,
		},
		{
			name:      "no glob characters",
			namespace: "upbound-system",
			pattern:   "upbound",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

	tests := []struct {
		name     string
		patterns []string
		want     []string
	}{
		{
			name:     "exact match single",
			patterns: []string{"upbound-system"},
			want:     []string{"upbound-system"},
		},
		{
			name:     "glob prefix single",
			patterns: []string{"upbound-*"},
			want:     []string{"upbound-system", "upbound-test"},
		},
		{
			name:     "glob suffix",
			patterns: []string{"*-system"},
			want:     []string{"upbound-system", "crossplane-system", "kube-system"},
		},
		{
			name:     "multiple exact patterns",
			patterns: []string{"upbound-system", "crossplane-system"},
			want:     []string{"upbound-system", "crossplane-system"},
		},
		{
			name:     "multiple glob patterns",
			patterns: []string{"upbound-*", "kube-*"},
			want:     []string{"upbound-system", "upbound-test", "kube-system"},
		},
		{
			name:     "mixed exact and glob",
			patterns: []string{"default", "upbound-*"},
			want:     []string{"default", "upbound-system", "upbound-test"},
		},
		{
			name:     "no matches",
			patterns: []string{"nonexistent-*"},
			want:     []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchNamespaces(namespaces, tt.patterns)
			if diff := cmp.Diff(tt.want, got, cmpopts.SortSlices(func(a, b string) bool {
				return a < b
			})); diff != "" {
				t.Errorf("%s\nmatchNamespaces(...): -want, +got\n%s", tt.name, diff)
			}
		})
	}
}

func TestShouldExcludeNamespace(t *testing.T) {
	tests := []struct {
		name            string
		namespace       string
		excludePatterns []string
		want            bool
	}{
		{
			name:            "exact match exclude",
			namespace:       "upbound-system",
			excludePatterns: []string{"upbound-system"},
			want:            true,
		},
		{
			name:            "exact match no exclude",
			namespace:       "upbound-system",
			excludePatterns: []string{"crossplane-system"},
			want:            false,
		},
		{
			name:            "glob prefix exclude",
			namespace:       "upbound-test",
			excludePatterns: []string{"upbound-*"},
			want:            true,
		},
		{
			name:            "glob prefix no exclude",
			namespace:       "crossplane-system",
			excludePatterns: []string{"upbound-*"},
			want:            false,
		},
		{
			name:            "multiple patterns match",
			namespace:       "upbound-test",
			excludePatterns: []string{"kube-*", "upbound-*"},
			want:            true,
		},
		{
			name:            "multiple patterns no match",
			namespace:       "default",
			excludePatterns: []string{"kube-*", "upbound-*"},
			want:            false,
		},
		{
			name:            "empty patterns",
			namespace:       "upbound-system",
			excludePatterns: []string{},
			want:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldExcludeNamespace(tt.namespace, tt.excludePatterns)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("%s\nshouldExcludeNamespace(%q, %v): -want, +got\n%s", tt.name, tt.namespace, tt.excludePatterns, diff)
			}
		})
	}
}
