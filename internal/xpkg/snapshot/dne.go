// Copyright 2025 Upbound Inc.
// All rights reserved

package snapshot

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/upbound/up/internal/xpkg/snapshot/validator"
)

const (
	warnNoDefinitionFound = "no definition found for resource"
)

// gvkDNEWarning returns a Validation indicating warning that a validator
// could not be found for the given gvk. Location is provided to indicate
// where the warning should be surfaced.
func gvkDNEWarning(gvk schema.GroupVersionKind, location string) []error {
	return []error{
		&validator.ValidationError{
			TypeCode: validator.WarningTypeCode,
			Message:  fmt.Sprintf(errFmt, warnNoDefinitionFound, gvk),
			Name:     location,
		},
	}
}
