// Copyright 2025 Upbound Inc.
// All rights reserved

package tar

import (
	"archive/tar"
	"bytes"
	"encoding/json"

	usagejson "github.com/upbound/up/internal/usage/encoding/json"
	"github.com/upbound/up/internal/usage/model"
	"github.com/upbound/up/internal/usage/report"
)

const (
	metaFilename  = "report/meta.json"
	usageFilename = "report/usage.json"
	mode          = 0o644
)

// Writer writes Upbound usage events for a single account to a usage report in
// a tar archive. Must be initialized with NewWriter(). Callers must call
// Close() on the writer when finished writing to it.
type Writer struct {
	tw   *tar.Writer
	meta report.Meta
	ee   *usagejson.MXPGVKEventEncoder
	buf  *bytes.Buffer
}

// NewWriter returns an initialized *Writer.
func NewWriter(tw *tar.Writer, meta report.Meta) (*Writer, error) {
	buf := &bytes.Buffer{}
	ue, err := usagejson.NewMXPGVKEventEncoder(buf)
	if err != nil {
		return nil, err
	}
	return &Writer{tw: tw, meta: meta, ee: ue, buf: buf}, nil
}

// Write writes an Upbound usage event to a tar archive.
func (w *Writer) Write(e model.MXPGVKEvent) error {
	e.Tags.UpboundAccount = w.meta.UpboundAccount
	return w.ee.Encode(e)
}

// Close closes the writer.
func (w *Writer) Close() error {
	if err := w.ee.Close(); err != nil {
		return err
	}
	if err := writeMeta(w.tw, w.meta); err != nil {
		return err
	}
	return writeUsage(w.tw, w.buf.Bytes())
}

// writeMeta writes usage report metadata to a *tar.Writer.
func writeMeta(tw *tar.Writer, meta report.Meta) error {
	b, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	if err := tw.WriteHeader(&tar.Header{
		Name: metaFilename,
		Mode: mode,
		Size: int64(len(b)),
	}); err != nil {
		return err
	}
	_, err = tw.Write(b)
	return err
}

// writeUsage writes usage data to a *tar.Writer.
func writeUsage(tw *tar.Writer, b []byte) error {
	if err := tw.WriteHeader(&tar.Header{
		Name: usageFilename,
		Mode: mode,
		Size: int64(len(b)),
	}); err != nil {
		return err
	}
	_, err := tw.Write(b)
	return err
}
