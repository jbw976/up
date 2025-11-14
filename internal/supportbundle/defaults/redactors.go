// Copyright 2025 Upbound Inc.
// All rights reserved

// Package defaults provides default configurations for support bundles.
package defaults

import (
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

// Redactors returns the default redactors.
func Redactors() *troubleshootv1beta2.Redactor {
	return &troubleshootv1beta2.Redactor{
		Spec: troubleshootv1beta2.RedactorSpec{
			Redactors: []*troubleshootv1beta2.Redact{
				{
					// Redact API keys from Spaces OTEL collectors
					Name: "api-key-redactor",
					Removals: troubleshootv1beta2.Removals{
						Regex: []troubleshootv1beta2.Regex{
							{
								Redactor: `.*"api-key"\s*:.*`,
							},
							{
								Redactor: `.*api-key:\s+.*`,
							},
						},
					},
				},
				{
					// Redact IPv4 addresses in logs only (exclude cluster-resources to preserve pod/service IPs)
					// Using pattern matching that targets .log files but excludes cluster-resources JSON
					Name: "Redact ipv4 addresses",
					FileSelector: troubleshootv1beta2.FileSelector{
						Files: []string{"**/*.log"},
					},
					Removals: troubleshootv1beta2.Removals{
						Regex: []troubleshootv1beta2.Regex{
							{
								Redactor: `(?P<mask>\b(?P<drop>25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(?P<drop>25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(?P<drop>25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(?P<drop>25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\b)`,
							},
						},
					},
				},
				// Note: ConfigMap and EnvironmentConfig data redaction is handled via custom
				// post-processing in collect.go (redactSensitiveData function) to preserve
				// JSON format and properly handle nested structures.
			},
		},
	}
}
