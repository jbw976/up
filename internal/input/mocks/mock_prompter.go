// Copyright 2025 Upbound Inc.
// All rights reserved

// Package mocks contains a mock prompter implementation for use in tests.
package mocks

import "github.com/crossplane/crossplane-runtime/pkg/errors"

const (
	// ErrCannotPrompt is returned by the mock prompter.
	ErrCannotPrompt = "cannot prompt in non-interactive terminal"
)

// MockPrompter is a mock prompter for use in tests.
type MockPrompter struct{}

// Prompt prompts the user, but always returns an error.
func (m *MockPrompter) Prompt(label string, _ bool) (string, error) {
	return label, errors.New(ErrCannotPrompt)
}
