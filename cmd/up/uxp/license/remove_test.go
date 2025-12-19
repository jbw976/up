// Copyright 2025 Upbound Inc.
// All rights reserved

package license

import (
	"testing"

	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/upbound/controller-manager/apis/licensing/v1alpha1"
	"github.com/upbound/up/internal/upterm"
)

func TestRemove(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		existingLicense v1alpha1.License
		wantLicense     v1alpha1.License
	}{
		"Community": {
			existingLicense: v1alpha1.License{
				TypeMeta: metav1.TypeMeta{
					APIVersion: v1alpha1.LicenseGroupVersionKind.GroupVersion().String(),
					Kind:       v1alpha1.LicenseKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: v1alpha1.LicenseName,
				},
				Spec: v1alpha1.LicenseSpec{},
			},
			wantLicense: v1alpha1.License{
				TypeMeta: metav1.TypeMeta{
					APIVersion: v1alpha1.LicenseGroupVersionKind.GroupVersion().String(),
					Kind:       v1alpha1.LicenseKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: v1alpha1.LicenseName,
				},
				Spec: v1alpha1.LicenseSpec{},
			},
		},
		"Licensed": {
			existingLicense: v1alpha1.License{
				TypeMeta: metav1.TypeMeta{
					APIVersion: v1alpha1.LicenseGroupVersionKind.GroupVersion().String(),
					Kind:       v1alpha1.LicenseKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: v1alpha1.LicenseName,
				},
				Spec: v1alpha1.LicenseSpec{
					SecretRef: &v1alpha1.LicenseSecretRef{
						Namespace: "crossplane-system",
						Name:      "my-license",
					},
				},
			},
			wantLicense: v1alpha1.License{
				TypeMeta: metav1.TypeMeta{
					APIVersion: v1alpha1.LicenseGroupVersionKind.GroupVersion().String(),
					Kind:       v1alpha1.LicenseKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: v1alpha1.LicenseName,
				},
				Spec: v1alpha1.LicenseSpec{},
			},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			objects := []client.Object{&tc.existingLicense}
			if tc.existingLicense.Spec.SecretRef != nil {
				objects = append(objects, &corev1.Secret{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Secret",
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: tc.existingLicense.Spec.SecretRef.Namespace,
						Name:      tc.existingLicense.Spec.SecretRef.Name,
					},
					Type: corev1.SecretTypeOpaque,
				})
			}

			sch := runtime.NewScheme()
			_ = corev1.AddToScheme(sch)
			_ = v1alpha1.AddToScheme(sch)
			cl := fake.NewClientBuilder().
				WithScheme(sch).
				WithObjects(objects...).
				Build()

			c := &removeCmd{
				Force: true,
			}

			err := c.Run(cl, upterm.NewTestPrinter())
			assert.NilError(t, err)

			var gotLicense v1alpha1.License
			err = cl.Get(t.Context(), types.NamespacedName{Name: v1alpha1.LicenseName}, &gotLicense)
			assert.NilError(t, err)
			assert.DeepEqual(t, gotLicense.Spec, tc.wantLicense.Spec)

			if tc.existingLicense.Spec.SecretRef != nil {
				var gotSecret corev1.Secret
				secretNN := types.NamespacedName{
					Namespace: tc.existingLicense.Spec.SecretRef.Namespace,
					Name:      tc.existingLicense.Spec.SecretRef.Name,
				}
				err = cl.Get(t.Context(), secretNN, &gotSecret)
				assert.Assert(t, kerrors.IsNotFound(err))
			}
		})
	}
}
