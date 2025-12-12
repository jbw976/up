// Copyright 2025 Upbound Inc.
// All rights reserved

package migration

// DefaultSpinner is the default spinner constructor used by all the other
// subpackages, by default it's just a no-op.
var DefaultSpinner = func(msg string) Spinner {
	return noopSpinner{}
}

// noopSpinner is a spinner that does nothing.
type noopSpinner struct{}

func (noopSpinner) Start() {}

func (noopSpinner) Success() {}

func (noopSpinner) Fail() {}

func (noopSpinner) UpdateText(_ string) {}

func (noopSpinner) Logf(_ string, _ ...any) {}

// Spinner is an interface for creating Printers.
type Spinner interface {
	Start()
	Success()
	Fail()
	UpdateText(string)
	Logf(string, ...any)
}
