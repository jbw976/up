// Copyright 2025 Upbound Inc.
// All rights reserved

// Package gcp contains a usage implementation using GCP infrastructure.
package gcp

import (
	"compress/gzip"
	"context"
	"io"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/usage/encoding/json"
	"github.com/upbound/up/internal/usage/event"
	"github.com/upbound/up/internal/usage/model"
)

// ErrEOF indicates EOF.
var ErrEOF = event.ErrEOF

var _ event.Reader = &QueryEventReader{}

// QueryEventReader is an event reader.
type QueryEventReader struct {
	Bucket *storage.BucketHandle
	Query  *storage.Query
	reader *ObjectIteratorEventReader
}

func (r *QueryEventReader) Read(ctx context.Context) (model.MXPGVKEvent, error) {
	if r.reader == nil {
		r.reader = &ObjectIteratorEventReader{Bucket: r.Bucket, Iterator: r.Bucket.Objects(ctx, r.Query)}
	}
	return r.reader.Read(ctx)
}

// Close closes the reader.
func (r *QueryEventReader) Close() error {
	if r.reader == nil {
		return nil
	}
	return r.reader.Close()
}

var _ event.Reader = &ObjectIteratorEventReader{}

// ObjectIteratorEventReader is an event reader.
type ObjectIteratorEventReader struct {
	Bucket     *storage.BucketHandle
	Iterator   *storage.ObjectIterator
	currReader *ObjectHandleEventReader
}

func (r *ObjectIteratorEventReader) Read(ctx context.Context) (model.MXPGVKEvent, error) {
	for {
		if r.currReader == nil {
			attrs, err := r.Iterator.Next()
			if errors.Is(err, iterator.Done) {
				return model.MXPGVKEvent{}, ErrEOF
			}
			r.currReader = &ObjectHandleEventReader{Object: r.Bucket.Object(attrs.Name), Attrs: attrs}
		}
		if e, err := r.currReader.Read(ctx); !errors.Is(err, ErrEOF) {
			return e, err
		}
		if err := r.currReader.Close(); err != nil {
			return model.MXPGVKEvent{}, err
		}
		r.currReader = nil
	}
}

// Close closes the reader.
func (r *ObjectIteratorEventReader) Close() error {
	if r.currReader == nil {
		return nil
	}
	return r.currReader.Close()
}

var _ event.Reader = &ObjectHandleEventReader{}

// ObjectHandleEventReader is an event reader.
type ObjectHandleEventReader struct {
	Object  *storage.ObjectHandle
	Attrs   *storage.ObjectAttrs
	decoder *json.MXPGVKEventDecoder
	closers []io.Closer
}

func (r *ObjectHandleEventReader) Read(ctx context.Context) (model.MXPGVKEvent, error) {
	if r.decoder == nil {
		reader, err := r.Object.NewReader(ctx)
		if err != nil {
			return model.MXPGVKEvent{}, err
		}

		contentType := ""
		if r.Attrs != nil {
			contentType = r.Attrs.ContentType
		}

		var body io.ReadCloser
		switch contentType {
		case "application/gzip":
			fallthrough
		case "application/x-gzip":
			r.closers = append(r.closers, reader)
			body, err = gzip.NewReader(reader)
			if err != nil {
				return model.MXPGVKEvent{}, err
			}
		default:
			body = reader
		}
		r.closers = append(r.closers, body)

		decoder, err := json.NewMXPGVKEventDecoder(body)
		if err != nil {
			return model.MXPGVKEvent{}, err
		}
		r.decoder = decoder
	}
	if !r.decoder.More() {
		return model.MXPGVKEvent{}, ErrEOF
	}
	return r.decoder.Decode()
}

// Close closes the reader.
func (r *ObjectHandleEventReader) Close() error {
	// Close closers in reverse.
	for i := len(r.closers) - 1; i >= 0; i-- {
		if err := r.closers[i].Close(); err != nil {
			return err
		}
	}
	return nil
}
