// Copyright 2025 Upbound Inc.
// All rights reserved

package license

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	adminv1alpha1 "github.com/upbound/up-sdk-go/apis/admin/v1alpha1"
	"github.com/upbound/up/internal/upterm"

	_ "embed"
)

//go:embed show.tmpl
var tmpl string

// showCmd is the `up space license show` command.
type showCmd struct{}

// Run is the body of the command.
func (c *showCmd) Run(cl client.Client, printer upterm.Printer) error {
	ctx := context.Background()

	var l adminv1alpha1.SpaceLicense

	err := cl.Get(ctx, types.NamespacedName{Name: adminv1alpha1.SpaceLicenseName}, &l)
	if err != nil {
		if kerrors.IsNotFound(err) {
			return errors.New("space license not found")
		}

		return errors.Wrap(err, "failed to get license")
	}

	data := map[string]any{
		"SpaceLicense": l,
	}
	if l.Spec.SecretRef != nil && l.Spec.SecretRef.Name != "" && l.Spec.SecretRef.Namespace != "" {
		secret := &corev1.Secret{}

		err := cl.Get(ctx, types.NamespacedName{Name: l.Spec.SecretRef.Name, Namespace: l.Spec.SecretRef.Namespace}, secret)
		if err != nil {
			return errors.Wrap(err, "failed to get license secret")
		}

		key := adminv1alpha1.SpaceLicenseSecretKeyDefault
		if l.Spec.SecretRef.Key != "" {
			key = l.Spec.SecretRef.Key
		}

		d, ok := secret.Data[key]
		if !ok {
			return fmt.Errorf("license secret is missing key: %s", key)
		}
		// decode the license from json
		var license map[string]any

		if err := json.Unmarshal(d, &license); err != nil {
			return errors.Wrap(err, "failed to unmarshal license data")
		}

		data["license"] = license
	}

	if err := printer.PrintObjectTemplate(&data, tmpl); err != nil {
		return errors.Wrap(err, "failed to show license")
	}

	return nil
}
