// Copyright 2025 Upbound Inc.
// All rights reserved

// Package style contains the shared style for the Upbound CLI.
package style

import (
	"os"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"golang.org/x/term"

	_ "embed"
)

var (
	// UpboundBrandColor is the Upbound brand color.
	//
	//nolint:gochecknoglobals // We'd make these consts if we could.
	UpboundBrandColor = lipgloss.AdaptiveColor{Light: "#5e3ba5", Dark: "#af7efd"}
	// NeutralColor is the neutral color.
	//
	//nolint:gochecknoglobals // We'd make these consts if we could.
	NeutralColor = lipgloss.AdaptiveColor{Light: "#4e5165", Dark: "#9a9ca7"}
	// DimColor is the dim color.
	//
	//nolint:gochecknoglobals // We'd make these consts if we could.
	DimColor = lipgloss.AdaptiveColor{Light: "#9B9B9B", Dark: "#5C5C5C"}
)

var (
	// SelectableItemStyle is the selectable item style.
	//
	//nolint:gochecknoglobals // We'd make these consts if we could.
	SelectableItemStyle = lipgloss.NewStyle().Foreground(DimColor)
	// KindStyle is the kind style.
	//
	//nolint:gochecknoglobals // We'd make these consts if we could.
	KindStyle = lipgloss.NewStyle().Foreground(NeutralColor)
	// SelectedItemStyle is the selected item style.
	//
	//nolint:gochecknoglobals // We'd make these consts if we could.
	SelectedItemStyle = lipgloss.NewStyle().Foreground(UpboundBrandColor)
)

// UpboundRootStyle is the Upbound root style.
//
//nolint:gochecknoglobals // We'd make these consts if we could.
var UpboundRootStyle = lipgloss.NewStyle().Foreground(UpboundBrandColor)

// RenderMarkdown formats markdown-formatted text for output to the terminal. If
// anything fails, the raw markdown is returned.
func RenderMarkdown(md string) string {
	wrapWidth, _, _ := term.GetSize(int(os.Stdout.Fd()))
	wrapWidth = min(wrapWidth, 120)

	tr, err := glamour.NewTermRenderer(
		getStyleOpt(),
		glamour.WithWordWrap(wrapWidth),
	)
	if err != nil {
		return md
	}

	formatted, err := tr.Render(md)
	if err != nil {
		return md
	}

	return formatted
}

var (
	//go:embed light.json
	lightStylesheet []byte
	//go:embed dark.json
	darkStylesheet []byte
)

func getStyleOpt() glamour.TermRendererOption {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return glamour.WithStandardStyle(styles.AsciiStyle)
	}

	if termenv.HasDarkBackground() {
		return glamour.WithStylesFromJSONBytes(darkStylesheet)
	}

	return glamour.WithStylesFromJSONBytes(lightStylesheet)
}
