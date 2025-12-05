// Copyright 2025 Upbound Inc.
// All rights reserved

package upterm

import (
	"os"

	"github.com/charmbracelet/huh"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
)

// Confirm shows a confirmation prompt and returns the result.
func Confirm(text string, def bool) (bool, error) {
	conf := huh.NewConfirm().
		Title(text).
		Inline(true).
		Value(&def).
		WithTheme(theme())

	err := conf.Run()
	if errors.Is(err, huh.ErrUserAborted) {
		os.Exit(1)
	}

	echoField(conf)

	return def, err
}
