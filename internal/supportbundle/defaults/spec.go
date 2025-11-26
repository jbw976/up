// Copyright 2025 Upbound Inc.
// All rights reserved

package defaults

import (
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

// SupportBundleSpec creates the default SupportBundle spec with the given namespaces.
// If includeLogs is false, log collectors are excluded (useful for Crossplane-only bundles).
func SupportBundleSpec(namespaces []string, includeLogs bool) *troubleshootv1beta2.SupportBundleSpec {
	collectors := []*troubleshootv1beta2.Collect{}

	if includeLogs {
		for _, ns := range namespaces {
			collectors = append(collectors, &troubleshootv1beta2.Collect{
				Logs: &troubleshootv1beta2.Logs{
					Namespace: ns,
				},
			})
		}
	}

	collectors = append(collectors,
		&troubleshootv1beta2.Collect{
			ClusterInfo: &troubleshootv1beta2.ClusterInfo{},
		},
		&troubleshootv1beta2.Collect{
			ClusterResources: &troubleshootv1beta2.ClusterResources{
				Namespaces: namespaces,
			},
		})

	return &troubleshootv1beta2.SupportBundleSpec{
		Collectors: collectors,
		// TODO: Add analyzers
		Analyzers: []*troubleshootv1beta2.Analyze{},
	}
}
