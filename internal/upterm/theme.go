// Copyright 2025 Upbound Inc.
// All rights reserved

package upterm

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/muesli/termenv"
	"golang.org/x/term"

	"github.com/upbound/up/internal/style"
)

func theme() *huh.Theme {
	if termenv.HasDarkBackground() {
		return darkTheme()
	}

	return lightTheme()
}

func darkTheme() *huh.Theme {
	th := huh.ThemeBase()
	f := &th.Focused
	b := &th.Blurred

	// No left border. For some reason, getting rid of left padding entirely
	// breaks echo, so we leave it alone.
	f.Base = f.Base.BorderLeft(false)
	b.Base = b.Base.PaddingLeft(0)

	// Confirm
	f.FocusedButton = f.FocusedButton.
		Background(style.UpboundBrandColor)
	b.FocusedButton = b.FocusedButton.
		Background(style.UpboundBrandColor)

	// Selection
	f.SelectSelector = f.SelectSelector.
		Foreground(style.UpboundBrandColor)
	b.SelectSelector = b.SelectSelector.
		Foreground(style.UpboundBrandColor)

	// Multi-selection
	f.MultiSelectSelector = f.MultiSelectSelector.
		Foreground(style.UpboundBrandColor)
	f.SelectedPrefix = f.SelectedPrefix.
		Foreground(style.UpboundBrandColor)
	b.MultiSelectSelector = b.MultiSelectSelector.
		Foreground(style.UpboundBrandColor)
	b.SelectedPrefix = b.SelectedPrefix.
		Foreground(style.UpboundBrandColor)

	// Prompt
	f.TextInput.Text = f.TextInput.Text.
		Foreground(style.UpboundBrandColor)
	b.TextInput.Text = b.TextInput.Text.
		Foreground(style.UpboundBrandColor)

	return th
}

func lightTheme() *huh.Theme {
	th := huh.ThemeBase()
	f := &th.Focused
	b := &th.Blurred

	// No left border. For some reason, getting rid of left padding entirely
	// breaks echo, so we leave it alone.
	f.Base = f.Base.BorderLeft(false)
	b.Base = b.Base.PaddingLeft(0)

	// Confirm
	f.FocusedButton = f.FocusedButton.
		Background(style.UpboundBrandColor)
	b.FocusedButton = b.FocusedButton.
		Background(style.UpboundBrandColor)
	f.BlurredButton = f.BlurredButton.
		Reverse(true)
	b.BlurredButton = b.BlurredButton.
		Reverse(true)

	// Selection
	f.SelectSelector = f.SelectSelector.
		Foreground(style.UpboundBrandColor)
	b.SelectSelector = b.SelectSelector.
		Foreground(style.UpboundBrandColor)

	// Multi-selection
	f.MultiSelectSelector = f.MultiSelectSelector.
		Foreground(style.UpboundBrandColor)
	f.SelectedPrefix = f.SelectedPrefix.
		Foreground(style.UpboundBrandColor)
	b.MultiSelectSelector = b.MultiSelectSelector.
		Foreground(style.UpboundBrandColor)
	b.SelectedPrefix = b.SelectedPrefix.
		Foreground(style.UpboundBrandColor)

	// Prompt
	f.TextInput.Text = f.TextInput.Text.
		Foreground(style.UpboundBrandColor)
	b.TextInput.Text = b.TextInput.Text.
		Foreground(style.UpboundBrandColor)

	return th
}

func echoField(f huh.Field) {
	// The huh widgets pad themselves to the full width of the screen, which
	// results in some weird wrapping when we echo them (which we need to do
	// since they disappear after being filled in). Detect the width of the
	// terminal (default to 80) and force them to that width so we can print
	// them more reliably.

	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		w = 80
	}
	fmt.Println(f.WithWidth(w - 2).View()) //nolint:forbidigo // This is an output library.
}
