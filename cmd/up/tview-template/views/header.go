// Copyright 2025 Upbound Inc.
// All rights reserved

package views

import (
	"github.com/rivo/tview"

	"github.com/upbound/up/cmd/up/tview-template/style"
)

type Header struct {
	*tview.TextView
}

func NewHeader() *Header {
	return &Header{
		TextView: tview.NewTextView().
			SetTextAlign(tview.AlignLeft).
			SetText(" ↑↓ up/down   ←→ time   +- expand/collapse   enter,space toggle   a auto-collapse   tab focus   f zoom   t,T time-scale   F3 yaml   end now   q,F10 quit").
			SetTextColor(style.Header),
	}
}
