// Copyright 2025 Upbound Inc.
// All rights reserved

package license

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/controller-manager/apis/licensing/v1alpha1"
	"github.com/upbound/up/internal/upterm"
)

const tmpl = `{{- with .Status }}
{{- $validity := "Unknown" }}
{{- range .Conditions }}
{{- if eq .Type "LicenseValid" }}
{{- if eq .Status "True" }}
{{- $validity = printf "Valid (%s)" .Message }}
{{- else }}
{{- $validity = printf "Invalid (%s)" .Message }}
{{- end }}
{{- end }}
{{- end }}Upbound Crossplane License Status: 	{{ $validity }}

{{- if .CreatedAt }}
Created: 	{{ .CreatedAt }}
{{- end }}
{{- if .ExpiresAt }}
Expires: 	{{ .ExpiresAt }}
{{- end }}

Plan: 	{{ .Plan }}
{{- if .Capacity }}
Resource Hour Limit: 	{{ .Capacity.ResourceHours }}
Operation Limit: 	{{ .Capacity.Operations }}
{{- end }}
Enabled Features: {{- if not .EnabledFeatures }} 	None {{- end}}
{{- range .EnabledFeatures }}
- {{ . }}
{{- end }}
{{- end }}`

// showCmd is the `up uxp license show` command.
type showCmd struct{}

// Run is the body of the command.
func (c *showCmd) Run(cl client.Client, printer upterm.ObjectPrinter) error {
	var l v1alpha1.License
	if err := cl.Get(context.Background(), types.NamespacedName{Name: v1alpha1.LicenseName}, &l); err != nil {
		return errors.Wrap(err, "failed to get license")
	}

	if err := printer.PrintTemplate(l, tmpl); err != nil {
		return errors.Wrap(err, "failed to show license")
	}

	return nil
}
