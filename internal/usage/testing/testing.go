// Copyright 2025 Upbound Inc.
// All rights reserved

package testing

import (
	"context"
	"fmt"
	"sort"

	"github.com/upbound/up/internal/usage/event"
	"github.com/upbound/up/internal/usage/model"
	"github.com/upbound/up/internal/usage/time"
)

var ErrEOF = event.ErrEOF

// ReadResult is a return value of event.Reader.Read().
type ReadResult struct {
	Event model.MXPGVKEvent
	Err   error
}

var _ event.Reader = &MockReader{}

type MockReader struct {
	Reads []ReadResult
}

func (r *MockReader) Read(context.Context) (model.MXPGVKEvent, error) {
	if len(r.Reads) < 1 {
		return model.MXPGVKEvent{}, ErrEOF
	}
	read := r.Reads[0]
	r.Reads = r.Reads[1:]
	return read.Event, read.Err
}

func (r *MockReader) Close() error {
	return nil
}

// Window is a return value of event.WindowIterator.Next().
type Window struct {
	Reader event.Reader
	Window time.Range
	Err    error
}

var _ event.WindowIterator = &MockWindowIterator{}

type MockWindowIterator struct {
	Windows []Window
}

func (i *MockWindowIterator) More() bool {
	return len(i.Windows) > 0
}

func (i *MockWindowIterator) Next() (event.Reader, time.Range, error) {
	if !i.More() {
		return nil, time.Range{}, fmt.Errorf("iterator is done")
	}
	w := i.Windows[0]
	i.Windows = i.Windows[1:]
	return w.Reader, w.Window, w.Err
}

var _ event.Writer = &MockWriter{}

type MockWriter struct {
	Events []model.MXPGVKEvent
}

func (w *MockWriter) Write(e model.MXPGVKEvent) error {
	if w.Events == nil {
		w.Events = []model.MXPGVKEvent{}
	}
	w.Events = append(w.Events, e)
	return nil
}

// SortEvents sorts events by their fields.
func SortEvents(events []model.MXPGVKEvent) {
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].Name != events[j].Name {
			return events[i].Name < events[j].Name
		}
		if events[i].Tags.UpboundAccount != events[j].Tags.UpboundAccount {
			return events[i].Tags.UpboundAccount < events[j].Tags.UpboundAccount
		}
		if events[i].Tags.MXPID != events[j].Tags.MXPID {
			return events[i].Tags.MXPID < events[j].Tags.MXPID
		}
		if events[i].Tags.Group != events[j].Tags.Group {
			return events[i].Tags.Group < events[j].Tags.Group
		}
		if events[i].Tags.Version != events[j].Tags.Version {
			return events[i].Tags.Version < events[j].Tags.Version
		}
		if events[i].Tags.Kind != events[j].Tags.Kind {
			return events[i].Tags.Kind < events[j].Tags.Kind
		}
		return events[i].Value < events[j].Value
	})
}
