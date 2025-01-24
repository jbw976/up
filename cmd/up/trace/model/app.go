// Copyright 2025 Upbound Inc.
// All rights reserved

package model

import (
	"sync/atomic"
	"time"

	"github.com/upbound/up/cmd/up/query"
	"github.com/upbound/up/internal/tview/model"
)

const DefaultScale = time.Second * 10

var Scales = []time.Duration{
	time.Second * 10,
	time.Second * 30,
	time.Minute,
	time.Minute * 5,
	time.Minute * 15,
	time.Minute * 30,
}

type App struct {
	TopLevel model.TopLevel
	Tree     Tree
	TimeLine TimeLine
	Zoomed   bool

	Resources      atomic.Pointer[[]string]
	GroupKindNames atomic.Pointer[query.GroupKindNames]
	CategoryNames  atomic.Pointer[query.CategoryNames]
}

func NewApp(resources []string, gkns query.GroupKindNames, cns query.CategoryNames) *App {
	a := &App{
		Tree: NewTree(),
		TimeLine: TimeLine{
			Scale: DefaultScale,
		},
	}

	a.Resources.Store(&resources)
	a.GroupKindNames.Store(&gkns)
	a.CategoryNames.Store(&cns)

	return a
}
