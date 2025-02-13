// Copyright 2025 Upbound Inc.
// All rights reserved

// Package reader contains an event reader.
package reader

import (
	"context"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/usage/event"
	"github.com/upbound/up/internal/usage/model"
)

// ErrEOF indicates EOF.
var ErrEOF = event.ErrEOF

var _ event.Reader = &MultiReader{}

// MultiReader is the logical concatenation of its readers. They're read
// sequentially. Once all readers have returned EOF, Read will return EOF. If
// any of the readers return a non-nil, non-EOF error, Read will return that
// error. Readers are closed when they return EOF or when Close() is called.
type MultiReader struct {
	Readers []event.Reader
}

func (r *MultiReader) Read(ctx context.Context) (model.MXPGVKEvent, error) {
	for {
		if len(r.Readers) < 1 {
			return model.MXPGVKEvent{}, ErrEOF
		}
		er := r.Readers[0]
		e, err := er.Read(ctx)
		if !errors.Is(err, ErrEOF) {
			return e, err
		}
		if err := er.Close(); err != nil {
			return model.MXPGVKEvent{}, err
		}
		r.Readers = r.Readers[1:]
	}
}

// Close closes the reader.
func (r *MultiReader) Close() error {
	for _, er := range r.Readers {
		if err := er.Close(); err != nil {
			return err
		}
	}
	return nil
}
