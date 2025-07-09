// Copyright 2025 Upbound Inc.
// All rights reserved

package license

import (
	"bytes"
	"testing"
	"text/template"
	"time"

	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpcommonv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/controller-manager/apis/licensing/v1alpha1"
)

// TestShowTemplate verifies that the template we use for pretty-printing
// licenses is valid and produces the expected output.
func TestShowTemplate(t *testing.T) {
	t.Parallel()

	var (
		mockCreatedAt = metav1.NewTime(time.Date(2023, 0o2, 15, 9, 15, 0, 0, time.UTC))
		mockExpiresAt = metav1.NewTime(time.Date(2025, 0o7, 29, 21, 45, 0, 0, time.UTC))
	)

	parsedTmpl, err := template.New("show").Parse(tmpl)
	assert.NilError(t, err)

	tcs := map[string]struct {
		license v1alpha1.License
		want    string
	}{
		"ValidCommunityEdition": {
			license: v1alpha1.License{
				Status: v1alpha1.LicenseStatus{
					Plan: v1alpha1.PlanCommunity,
					ConditionedStatus: xpcommonv1.ConditionedStatus{
						Conditions: []xpcommonv1.Condition{v1alpha1.LicenseCommunityEdition()},
					},
				},
			},
			want: `Upbound Crossplane License Status: Valid (Community edition license is active.)

Plan: community
Enabled Features: None`,
		},
		"ValidCommercial": {
			license: v1alpha1.License{
				Status: v1alpha1.LicenseStatus{
					Plan:              "commercial",
					CreatedAt:         &mockCreatedAt,
					ExpiresAt:         &mockExpiresAt,
					GracePeriodEndsAt: &mockExpiresAt,
					EnabledFeatures: []string{
						"Cool feature 1",
						"Cool Feature 2",
					},
					Capacity: &v1alpha1.LicenseCapacity{
						ResourceHours: 1000,
						Operations:    1000,
					},
					ConditionedStatus: xpcommonv1.ConditionedStatus{
						Conditions: []xpcommonv1.Condition{v1alpha1.LicenseValid()},
					},
				},
			},
			want: `Upbound Crossplane License Status: Valid (The license signature has been successfully verified.)
Created: 2023-02-15 09:15:00 +0000 UTC
Expires: 2025-07-29 21:45:00 +0000 UTC

Plan: commercial
Resource Hour Limit: 1000
Operation Limit: 1000
Enabled Features:
- Cool feature 1
- Cool Feature 2`,
		},
		"Invalid": {
			license: v1alpha1.License{
				Status: v1alpha1.LicenseStatus{
					Plan:              "commercial",
					CreatedAt:         &mockCreatedAt,
					ExpiresAt:         &mockExpiresAt,
					GracePeriodEndsAt: &mockExpiresAt,
					EnabledFeatures: []string{
						"Cool feature 1",
						"Cool Feature 2",
					},
					Capacity: &v1alpha1.LicenseCapacity{
						ResourceHours: 1000,
						Operations:    1000,
					},
					ConditionedStatus: xpcommonv1.ConditionedStatus{
						Conditions: []xpcommonv1.Condition{v1alpha1.LicenseInvalid(errors.New("stolen crossplane"))},
					},
				},
			},
			want: `Upbound Crossplane License Status: Invalid (License validation failed: stolen crossplane)
Created: 2023-02-15 09:15:00 +0000 UTC
Expires: 2025-07-29 21:45:00 +0000 UTC

Plan: commercial
Resource Hour Limit: 1000
Operation Limit: 1000
Enabled Features:
- Cool feature 1
- Cool Feature 2`,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			err := parsedTmpl.Execute(&buf, tc.license)
			assert.NilError(t, err)
			assert.Equal(t, buf.String(), tc.want)
		})
	}
}
