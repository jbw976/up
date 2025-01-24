// Copyright 2025 Upbound Inc.
// All rights reserved

package views

import (
	"github.com/rivo/tview"
)

type ExampleTextView struct {
	*tview.TextView
}

func NewExampleTextView() *ExampleTextView {
	d := &ExampleTextView{
		TextView: tview.NewTextView().SetText("Hello World"),
	}
	return d
}
