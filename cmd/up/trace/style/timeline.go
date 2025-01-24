// Copyright 2025 Upbound Inc.
// All rights reserved

package style

import (
	"github.com/gdamore/tcell/v2"
)

type TimeLine struct {
	Earlier tcell.Style
	Later   tcell.Style

	NotExisting tcell.Style
	Deleted     tcell.Style
	NotSynced   tcell.Style
	NotReady    tcell.Style
	Ready       tcell.Style
}

var (
	DefaultTimelineStyle = TimeLine{
		Earlier: tcell.StyleDefault.Foreground(tcell.GetColor("#17e1cf")),
		Later:   tcell.StyleDefault.Foreground(tcell.GetColor("#17e1cf")),

		NotExisting: tcell.StyleDefault,
		Deleted:     tcell.StyleDefault.Background(tcell.GetColor("#303030")),
		NotSynced:   tcell.StyleDefault.Background(tcell.GetColor("#805056")),
		NotReady:    tcell.StyleDefault.Background(tcell.GetColor("#80672c")),
		Ready:       tcell.StyleDefault.Background(tcell.GetColor("#0c7568")),
	}

	SelectedTimeLineStyle = TimeLine{
		Earlier: tcell.StyleDefault.Foreground(tcell.GetColor("#17e1cf")),
		Later:   tcell.StyleDefault.Foreground(tcell.GetColor("#17e1cf")),

		NotExisting: tcell.StyleDefault,
		Deleted:     tcell.StyleDefault.Background(tcell.GetColor("#737373")),
		NotSynced:   tcell.StyleDefault.Background(tcell.GetColor("#996067")),
		NotReady:    tcell.StyleDefault.Background(tcell.GetColor("#997b34")),
		Ready:       tcell.StyleDefault.Background(tcell.GetColor("#0e8c7c")),
	}
)
