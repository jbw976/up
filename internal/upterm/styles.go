// Copyright 2025 Upbound Inc.
// All rights reserved

package upterm

import (
	"fmt"
	"io"

	"github.com/pterm/pterm"
)

var (
	// EyesPrefix is a prefix used for eye-related output.
	//nolint:gochecknoglobals // used anywhere
	EyesPrefix = pterm.Prefix{
		Style: &pterm.Style{pterm.FgLightMagenta},
		Text:  " 👀",
	}

	// RaisedPrefix is a prefix used for raised-hand output.
	//nolint:gochecknoglobals // used anywhere
	RaisedPrefix = pterm.Prefix{
		Style: &pterm.Style{pterm.FgLightMagenta},
		Text:  " 🙌",
	}

	//nolint:gochecknoglobals // used anywhere
	spinnerStyle = &pterm.Style{pterm.FgDarkGray}
	//nolint:gochecknoglobals // used anywhere
	msgStyle = &pterm.Style{pterm.FgDefault}

	// CheckmarkSuccessSpinner is the default spinner for success messages.
	//nolint:gochecknoglobals // used anywhere
	CheckmarkSuccessSpinner = pterm.DefaultSpinner.WithStyle(spinnerStyle).WithMessageStyle(msgStyle)

	// EyesInfoSpinner is the default spinner for informational messages.
	//nolint:gochecknoglobals // used anywhere
	EyesInfoSpinner = pterm.DefaultSpinner.WithStyle(spinnerStyle).WithMessageStyle(msgStyle)

	// ComponentText is the style for component text.
	//nolint:gochecknoglobals // used anywhere
	ComponentText = pterm.DefaultBasicText.WithStyle(&pterm.ThemeDefault.TreeTextStyle)
)

func successPrinter() *pterm.PrefixPrinter {
	return &pterm.PrefixPrinter{
		MessageStyle: &pterm.Style{pterm.FgDefault},
		Prefix: pterm.Prefix{
			Style: &pterm.Style{pterm.FgLightMagenta},
			Text:  " ✓ ",
		},
	}
}

func infoPrinter() *pterm.PrefixPrinter {
	return &pterm.PrefixPrinter{
		MessageStyle: &pterm.Style{pterm.FgDefault},
		Prefix:       EyesPrefix,
	}
}

func init() {
	CheckmarkSuccessSpinner.SuccessPrinter = successPrinter()
	EyesInfoSpinner.InfoPrinter = infoPrinter()
}

// WrapWithSuccessSpinner adds spinners around message and run function.
func WrapWithSuccessSpinner(msg string, spinner *pterm.SpinnerPrinter, f func() error, printer ObjectPrinter) error {
	if bool(printer.Quiet) {
		return f()
	}

	if !printer.Pretty {
		pterm.Printfln("%s …", msg)
		if err := f(); err != nil {
			pterm.Printfln("%s ✗ ", msg)
			return err
		}
		pterm.Printfln("%s ✓ ", msg)
		return nil
	}

	s, err := spinner.Start(msg)
	if err != nil {
		return err
	}

	if err := f(); err != nil {
		return err
	}

	s.Success()
	return nil
}

// StepCounter returns the counted steps.
func StepCounter(msg string, index, total int) string {
	return fmt.Sprintf("[%d/%d]: %s", index, total, msg)
}

// NewCheckmarkSuccessSpinner returns a new spinner that writes to the given
// writer and prints an Upbound-branded checkmark on success. This spinner will
// behave the same as the CheckmarkSuccessPrinter but multiple of them can be
// used at once (in a single thread - pterm is not concurrency-safe) since they
// don't share state.
func NewCheckmarkSuccessSpinner(w io.Writer) *pterm.SpinnerPrinter {
	sp := pterm.DefaultSpinner
	sp.SuccessPrinter = successPrinter()
	sp.Writer = w
	sp.MessageStyle = msgStyle
	sp.Style = spinnerStyle

	return &sp
}
