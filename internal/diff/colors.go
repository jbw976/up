// Copyright 2025 Upbound Inc.
// All rights reserved

package diff

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"github.com/upbound/up/internal/style"
)

// outputStyles defines how the output will be styled depending on the change
// type.
type outputStyles interface {
	Create(a ...any) string
	Update(a ...any) string
	Delete(a ...any) string
}

var _ outputStyles = &termColors{}

// termColors formats the output with terminal foreground colors.
type termColors struct {
	create lipgloss.Style
	update lipgloss.Style
	delete lipgloss.Style
}

func (c termColors) Create(v ...any) string {
	return c.create.Render(fmt.Sprint(v...))
}

func (c termColors) Update(v ...any) string {
	return c.update.Render(fmt.Sprint(v...))
}

func (c termColors) Delete(v ...any) string {
	return c.delete.Render(fmt.Sprint(v...))
}

func newDefaultTermColors() termColors {
	return termColors{
		create: lipgloss.NewStyle().Foreground(style.GreenColor),
		update: lipgloss.NewStyle().Foreground(style.YellowColor),
		delete: lipgloss.NewStyle().Foreground(style.RedColor),
	}
}

var _ outputStyles = &noColors{}

type noColors struct{}

func (noColors) Create(v ...any) string {
	return fmt.Sprint(v...)
}

func (noColors) Update(v ...any) string {
	return fmt.Sprint(v...)
}

func (noColors) Delete(v ...any) string {
	return fmt.Sprint(v...)
}
