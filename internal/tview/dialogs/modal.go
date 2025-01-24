// Copyright 2025 Upbound Inc.
// All rights reserved

package dialogs

import (
	"reflect"
	"unsafe"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func ShowModal(app *tview.Application, p tview.Primitive) *Modal {
	m := &Modal{
		Primitive:  p,
		app:        app,
		background: GetRoot(app),
	}
	app.SetRoot(m, true)
	return m
}

func GetRoot(app *tview.Application) tview.Primitive {
	fld := reflect.ValueOf(app).Elem().FieldByName("root")
	return *(*tview.Primitive)(unsafe.Pointer(fld.UnsafeAddr())) //nolint:gosec // no way around this
}

type Modal struct {
	tview.Primitive

	app        *tview.Application
	background tview.Primitive
}

func (m *Modal) Draw(screen tcell.Screen) {
	m.background.Draw(screen)
	m.Primitive.Draw(screen)
}

func (m *Modal) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	delegate := m.Primitive.InputHandler()
	return func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		if event.Key() == tcell.KeyEsc {
			m.app.SetRoot(m.background, true)
		}
		delegate(event, setFocus)
	}
}

func (m *Modal) Hide() {
	if GetRoot(m.app) == m.Primitive {
		m.app.SetRoot(m.background, true)
	}
}
