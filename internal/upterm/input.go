// Copyright 2025 Upbound Inc.
// All rights reserved

package upterm

import (
	"os"

	"github.com/charmbracelet/huh"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
)

// Prompt prompts the user for input and returns it.
func Prompt(text string, def string) (string, error) {
	in := huh.NewInput().
		Title(text).
		Value(&def).
		Inline(true).
		Prompt(" ").
		WithTheme(theme())

	err := in.Run()
	if errors.Is(err, huh.ErrUserAborted) {
		os.Exit(1)
	}

	echoField(in)

	return def, err
}
