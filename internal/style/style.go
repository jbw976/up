// Copyright 2025 Upbound Inc.
// All rights reserved

// Package style contains the shared style for the Upbound CLI.
package style

import "github.com/charmbracelet/lipgloss"

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
