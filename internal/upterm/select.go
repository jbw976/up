// Copyright 2025 Upbound Inc.
// All rights reserved

package upterm

import (
	"os"

	"github.com/charmbracelet/huh"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
)

// Selection presents the user with an interactive selection of choices and
// returns the selected choice.
func Selection(text string, choices []string, def string) (string, error) {
	sel := huh.NewSelect[string]().
		Title(text).
		Options(huh.NewOptions(choices...)...).
		Value(&def).
		WithTheme(theme())

	err := sel.Run()
	if errors.Is(err, huh.ErrUserAborted) {
		os.Exit(1)
	}

	echoField(sel)

	return def, err
}

// MultiSelection presents the user with an interactive selection of choices and
// returns the selected choices.
func MultiSelection(text string, choices []string, defs []string) ([]string, error) {
	if defs == nil {
		defs = []string{}
	}

	sel := huh.NewMultiSelect[string]().
		Title(text).
		Options(huh.NewOptions(choices...)...).
		Value(&defs).
		WithTheme(theme())

	err := sel.Run()
	if errors.Is(err, huh.ErrUserAborted) {
		os.Exit(1)
	}

	echoField(sel)

	return defs, err
}
