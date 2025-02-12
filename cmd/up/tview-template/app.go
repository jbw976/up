// Copyright 2025 Upbound Inc.
// All rights reserved

package template

import (
	"context"
	"net/http"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/upbound/up/cmd/up/tview-template/model"
	"github.com/upbound/up/cmd/up/tview-template/views"
	"github.com/upbound/up/internal/tview/dialogs"
	upviews "github.com/upbound/up/internal/tview/views"
)

// App represents main application struct.
type App struct {
	*tview.Application
	model *model.App

	header   *views.Header
	textView *views.ExampleTextView
	graph    *views.ExampleGraph

	grid     *tview.Grid
	topLevel *upviews.TopLevel
}

func NewApp(title string, client http.RoundTripper, kubeURL string) *App {
	app := &App{
		Application: tview.NewApplication(),
		model:       model.NewApp(),
	}

	app.header = views.NewHeader()
	app.textView = views.NewExampleTextView()
	app.graph = views.NewExampleGraph(client, kubeURL)

	app.grid = tview.NewGrid().
		SetRows(1, 0, 5).
		SetColumns(0).
		SetBorders(true).
		SetBordersColor(tcell.ColorDarkGray).
		AddItem(app.header, 0, 0, 1, 1, 0, 0, false).
		AddItem(app.textView, 1, 0, 1, 1, 0, 0, true).
		AddItem(app.graph, 2, 0, 1, 1, 0, 0, true)
	app.topLevel = upviews.NewTopLevel(title, app.grid, app.Application).
		SetTitles(upviews.GridTitle{Col: 0, Row: 1, Text: " ExampleTextView ", Color: tcell.ColorDarkGray, Align: tview.AlignCenter}).
		SetDelegateInputHandler(app.TopLevelInputHandler).
		SetCommands("", "ExampleTextView", "", "", "", "", "", "", "", "Quit")
	app.Application.SetRoot(app.topLevel, true)

	return app
}

func (a *App) TopLevelInputHandler(event *tcell.EventKey, setFocus func(p tview.Primitive)) bool {
	switch event.Key() { //nolint:exhaustive // there is a default case
	case tcell.KeyF2:
		oldRoot := dialogs.GetRoot(a.Application)
		dialogs.ShowModal(a.Application, dialogs.NewConfirmDialog().
			SetTitle("Congratulations").
			SetText("You pressed F2 🎉").
			SetCancelFunc(func() { a.Application.SetRoot(oldRoot, true) }).
			SetSelectedFunc(func() { a.Application.SetRoot(oldRoot, true) }).
			Display())
		return true
	case tcell.KeyRune:
		switch event.Rune() { // nolint:gocritic // this is an example to be extended with more cases
		case 'q':
			a.topLevel.InteractiveQuit()
		default:
		}
	default:
	}

	return false
}

func (a *App) Run(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		a.Application.Stop()
	}()

	go func() {
		for {
			time.Sleep(time.Second)
			err := a.graph.Tick(ctx) // nolint:errcheck
			if err != nil {
				a.textView.SetText(a.textView.GetText(false) + "\n" + err.Error())
			}
			a.QueueUpdateDraw(func() {})
		}
	}()

	return a.Application.Run()
}
