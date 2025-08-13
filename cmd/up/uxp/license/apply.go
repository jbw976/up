// Copyright 2025 Upbound Inc.
// All rights reserved

package license

import (
	"context"
	"io"

	"github.com/pterm/pterm"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/upbound/controller-manager/apis/licensing/v1alpha1"
	"github.com/upbound/up/internal/uxp"
)

type applyCmd struct {
	LicenseFile string `arg:""                                                                   help:"File containing the license key (required unless using --dev)." optional:"" type:"filepath" xor:"license-source"`
	Dev         bool   `help:"Apply embedded development license for single-node kind clusters." xor:"license-source"`

	Namespace string `default:"crossplane-system" help:"Namespace in which to create the license key secret."`

	// TODO(adamwg): fs is injectable for testing, but we don't have tests yet
	// because of
	// https://github.com/kubernetes-sigs/controller-runtime/issues/2341. Looks
	// like the fix is coming real soon now.
	fs afero.Fs
}

func (c *applyCmd) AfterApply() error {
	c.fs = afero.NewOsFs()

	// Validate mutual exclusivity of --dev and license file
	if c.Dev && c.LicenseFile != "" {
		return errors.New("cannot specify both --dev flag and license file")
	}
	if !c.Dev && c.LicenseFile == "" {
		return errors.New("must specify either --dev flag or license file")
	}

	return nil
}

func (c *applyCmd) Run(cl client.Client) error {
	ctx := context.Background()

	var licenseBytes []byte

	if c.Dev {
		licenseBytes = []byte(uxp.DevLicense)
	} else {
		f, err := c.fs.Open(c.LicenseFile)
		if err != nil {
			return errors.Wrap(err, "cannot open license file")
		}
		defer func() { _ = f.Close() }()

		licenseBytes, err = io.ReadAll(f)
		if err != nil {
			return errors.Wrap(err, "failed to read license")
		}
	}

	s := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: c.Namespace,
			Name:      v1alpha1.LicenseSecretName,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			v1alpha1.LicenseSecretKeyDefault: licenseBytes,
		},
	}

	l := &v1alpha1.License{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.LicenseGroupVersionKind.GroupVersion().String(),
			Kind:       v1alpha1.LicenseKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: v1alpha1.LicenseName,
		},
		Spec: v1alpha1.LicenseSpec{
			SecretRef: &v1alpha1.LicenseSecretRef{
				Namespace: s.GetNamespace(),
				Name:      s.GetName(),
				Key:       v1alpha1.LicenseSecretKeyDefault,
			},
		},
	}

	if err := cl.Patch(ctx, s, client.Apply, client.FieldOwner("up-cli"), client.ForceOwnership); err != nil {
		return errors.Wrap(err, "failed to apply license secret")
	}
	if err := cl.Patch(ctx, l, client.Apply, client.FieldOwner("up-cli"), client.ForceOwnership); err != nil {
		return errors.Wrap(err, "failed to apply license resource")
	}

	pterm.Println("Successfully applied license. Use `up uxp license show` to check license status.")

	return nil
}
