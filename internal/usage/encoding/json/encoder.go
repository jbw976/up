// Copyright 2025 Upbound Inc.
// All rights reserved

package json

import (
	"encoding/json"
	"io"

	"github.com/upbound/up/internal/usage/model"
)

// MXPGVKEventEncoder encodes MXP GVK events as a JSON array of event objects
// to a writer. Must be initialized with NewMXPGVKEventEncoder(). Callers must
// call Close() when finished encoding.
type MXPGVKEventEncoder struct {
	w              io.Writer
	wroteFirstItem bool
}

// NewMXPGVKEventEncoder returns an initialized *Encoder.
func NewMXPGVKEventEncoder(w io.Writer) (*MXPGVKEventEncoder, error) {
	// Write open bracket to open JSON array.
	if _, err := w.Write([]byte("[")); err != nil {
		return nil, err
	}
	return &MXPGVKEventEncoder{w: w}, nil
}

// Encode encodes and writes an MXP GVK event.
func (e *MXPGVKEventEncoder) Encode(event model.MXPGVKEvent) error {
	b := []byte{}

	if e.wroteFirstItem {
		// There's at least one preceding item, so print a comma.
		b = append(b, byte(','))
	}
	b = append(b, byte('\n'))

	eventBytes, err := json.Marshal(event)
	if err != nil {
		return err
	}
	b = append(b, eventBytes...)

	_, err = e.w.Write(b)
	if err == nil {
		e.wroteFirstItem = true
	}
	return err
}

// Close closes the encoder.
func (e *MXPGVKEventEncoder) Close() error {
	// Write close bracket to close JSON array.
	_, err := e.w.Write([]byte("\n]\n"))
	return err
}
