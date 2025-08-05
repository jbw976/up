// Copyright 2025 Upbound Inc.
// All rights reserved

package license

import (
	"context"
	"fmt"

	"github.com/pterm/pterm"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/controller-manager/apis/licensing/v1alpha1"
)

type removeCmd struct {
	Force bool `help:"Do not ask for confirmation before removing the license."`
}

func (c *removeCmd) Run(cl client.Client) error {
	ctx := context.Background()

	var l v1alpha1.License
	if err := cl.Get(ctx, types.NamespacedName{Name: v1alpha1.LicenseName}, &l); err != nil {
		return errors.Wrap(err, "failed to get license")
	}

	if l.Spec.SecretRef == nil {
		// Cluster is using the default community edition license.
		pterm.Println("Current license is the default community license, which cannot be removed.")
		return nil
	}

	// Confirm the user wants to remove their license.
	if !c.Force {
		confirmMsg := fmt.Sprintf("Are you sure you want to remove the license secret %s/%s?", l.Spec.SecretRef.Namespace, l.Spec.SecretRef.Name)
		proceed, err := pterm.DefaultInteractiveConfirm.Show(confirmMsg)
		if err != nil {
			return err
		}
		if !proceed {
			return errors.New("operation canceled")
		}
	}

	// Update the license to stop using the secret, then remove the secret.
	s := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: l.Spec.SecretRef.Namespace,
			Name:      l.Spec.SecretRef.Name,
		},
	}
	l.Spec.SecretRef = nil
	if err := cl.Update(ctx, &l, client.FieldOwner("up-cli")); err != nil {
		return errors.Wrap(err, "failed to update license resource")
	}
	if err := cl.Delete(ctx, s); err != nil && !kerrors.IsNotFound(err) {
		return errors.Wrap(err, "failed to delete license secret")
	}

	pterm.Println("Successfully removed license. Use `up uxp license show` to check license status.")

	return nil
}
