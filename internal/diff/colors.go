// Copyright 2025 Upbound Inc.
// All rights reserved

package diff

import (
	"fmt"

	"github.com/pterm/pterm"
)

const (
	// changeColorCreated is the color to use when displaying an created field.
	changeColorCreate = pterm.FgGreen

	// changeColorUpdate is the color to use when displaying an updated field.
	changeColorUpdate = pterm.FgYellow

	// changeColorDelete is the color to use when displaying an deleted field.
	changeColorDelete = pterm.FgRed
)

// outputStyles defines how the output will be styled depending on the change
// type.
type outputStyles interface {
	Create(...any) string
	Update(...any) string
	Delete(...any) string
}

var _ outputStyles = &termColors{}

// termColors formats the output with terminal foreground colors.
type termColors struct {
	create pterm.Color
	update pterm.Color
	delete pterm.Color
}

func (c termColors) Create(v ...any) string {
	return c.create.Sprint(v...)
}

func (c termColors) Update(v ...any) string {
	return c.update.Sprint(v...)
}

func (c termColors) Delete(v ...any) string {
	return c.delete.Sprint(v...)
}

func NewDefaultTermColors() termColors {
	return termColors{
		create: changeColorCreate,
		update: changeColorUpdate,
		delete: changeColorDelete,
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
