// Copyright 2025 Upbound Inc.
// All rights reserved

package composite

import (
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

// AsCompositionConnectionDetail translates a composition revision's connection
// detail to a composition connection detail.
func AsCompositionConnectionDetail(rcd v1.ConnectionDetail) v1.ConnectionDetail {
	return v1.ConnectionDetail{
		Name: rcd.Name,
		Type: func() *v1.ConnectionDetailType {
			if rcd.Type == nil {
				return nil
			}
			t := v1.ConnectionDetailType(*rcd.Type)
			return &t
		}(),
		FromConnectionSecretKey: rcd.FromConnectionSecretKey,
		FromFieldPath:           rcd.FromFieldPath,
		Value:                   rcd.Value,
	}
}

// AsCompositionReadinessCheck translates a composition revision's readiness
// check to a composition readiness check.
func AsCompositionReadinessCheck(rrc v1.ReadinessCheck) v1.ReadinessCheck {
	return v1.ReadinessCheck{
		Type:         v1.ReadinessCheckType(rrc.Type),
		FieldPath:    rrc.FieldPath,
		MatchString:  rrc.MatchString,
		MatchInteger: rrc.MatchInteger,
	}
}
