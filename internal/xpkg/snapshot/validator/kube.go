// Copyright 2025 Upbound Inc.
// All rights reserved

package validator

import (
	"context"

	"k8s.io/kube-openapi/pkg/validation/validate"
)

type kubeValidator interface {
	Validate(data any) *validate.Result
}

// NewUsingContext returns a new validator that uses the provided kubeValidator
// with no context.
func NewUsingContext(k kubeValidator) *UsingContext {
	return &UsingContext{
		k: k,
	}
}

// UsingContext allows us to use kube-openapi validators without context usage
// to conform our interfaces that require it.
type UsingContext struct {
	k kubeValidator
}

// Validate calls the underlying kubeValidator's Validate method without a context.
func (uc *UsingContext) Validate(_ context.Context, data any) *validate.Result {
	return uc.k.Validate(data)
}
