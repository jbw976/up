// Copyright 2025 Upbound Inc.
// All rights reserved

package generator

import (
	"context"

	"github.com/spf13/afero"

	"github.com/upbound/up/internal/schemas/runner"
)

// Interface generates schemas for a specific language.
type Interface interface {
	Language() string
	GenerateFromCRD(ctx context.Context, fs afero.Fs, runner runner.SchemaRunner) (afero.Fs, error)
	GenerateFromOpenAPI(ctx context.Context, fs afero.Fs, runner runner.SchemaRunner) (afero.Fs, error)
}

// AllLanguages returns generators for all supported languages.
func AllLanguages() []Interface {
	return []Interface{
		&goGenerator{},
		&jsonGenerator{},
		&kclGenerator{},
		&pythonGenerator{},
	}
}
