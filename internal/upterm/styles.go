// Copyright 2025 Upbound Inc.
// All rights reserved

package upterm

import (
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

	// BotPrefix is a prefix used for bot output.
	//nolint:gochecknoglobals // used anywhere
	BotPrefix = pterm.Prefix{
		Style: &pterm.Style{pterm.FgLightMagenta},
		Text:  " 🤖",
	}
)
