// Copyright 2025 Upbound Inc.
// All rights reserved

// Package event contains usage event types.
package event

import (
	"context"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/usage/model"
	"github.com/upbound/up/internal/usage/time"
)

// ErrEOF indicates EOF.
var ErrEOF = errors.New("EOF")

// Reader is the interface for reading usage events. Read() must return EOF when
// there is nothing more to read. Callers must call Close() when finished
// reading.
type Reader interface {
	// Read returns the next event. Returns EOF when finished.
	Read(ctx context.Context) (model.MXPGVKEvent, error)
	// Close closes the reader.
	Close() error
}

// WindowIterator is the interface for iterating through usage event readers for
// windows of time within a time range.
type WindowIterator interface {
	// More returns true if there are more windows.
	More() bool
	// Next returns a reader and time range for the next window.
	Next() (Reader, time.Range, error)
}

// Writer is the interface for reading usage events.
type Writer interface {
	// Write writes an event.
	Write(ev model.MXPGVKEvent) error
}
