// Copyright 2025 Upbound Inc.
// All rights reserved

package aws

import (
	"compress/gzip"
	"context"
	"io"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"

	"github.com/upbound/up/internal/usage/encoding/json"
	"github.com/upbound/up/internal/usage/event"
	"github.com/upbound/up/internal/usage/event/reader"
	"github.com/upbound/up/internal/usage/model"
)

var ErrEOF = event.ErrEOF

var _ event.Reader = &ListObjectsV2InputEventReader{}

// ListBlobsResponseEventReader reads usage events from a
// *s3.ListObjectsV2Input.
type ListObjectsV2InputEventReader struct {
	Client             *s3.S3
	Bucket             string
	ListObjectsV2Input *s3.ListObjectsV2Input
	reader             *reader.MultiReader
}

func (r *ListObjectsV2InputEventReader) Read(ctx context.Context) (model.MXPGVKEvent, error) {
	if r.reader == nil {
		readers := []event.Reader{}
		if err := r.Client.ListObjectsV2PagesWithContext(
			ctx,
			r.ListObjectsV2Input,
			func(page *s3.ListObjectsV2Output, _ bool) bool {
				for _, obj := range page.Contents {
					readers = append(readers, &GetObjectInputEventReader{
						Client: r.Client,
						GetObjectInput: &s3.GetObjectInput{
							Bucket: aws.String(r.Bucket),
							Key:    obj.Key,
						},
					})
				}
				return true
			},
		); err != nil {
			return model.MXPGVKEvent{}, err
		}
		r.reader = &reader.MultiReader{Readers: readers}
	}
	return r.reader.Read(ctx)
}

func (r *ListObjectsV2InputEventReader) Close() error {
	if r.reader == nil {
		return nil
	}
	return r.reader.Close()
}

var _ event.Reader = &GetObjectInputEventReader{}

// GetObjectInputEventReader reads usage events from a *s3.GetObjectInput.
type GetObjectInputEventReader struct {
	Client         *s3.S3
	GetObjectInput *s3.GetObjectInput
	decoder        *json.MXPGVKEventDecoder
	closers        []io.Closer
}

func (r *GetObjectInputEventReader) Read(ctx context.Context) (model.MXPGVKEvent, error) {
	if r.decoder == nil {
		// TODO(branden): Use s3manager.Downloader for streaming and concurrent
		// downloads.
		resp, err := r.Client.GetObjectWithContext(ctx, r.GetObjectInput)
		if err != nil {
			return model.MXPGVKEvent{}, err
		}

		contentType := ""
		if resp.ContentType != nil {
			contentType = *resp.ContentType
		}

		var body io.ReadCloser
		switch contentType {
		case "application/gzip":
			fallthrough
		case "application/x-gzip":
			r.closers = append(r.closers, resp.Body)
			body, err = gzip.NewReader(resp.Body)
			if err != nil {
				return model.MXPGVKEvent{}, err
			}
		default:
			body = resp.Body
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

func (r *GetObjectInputEventReader) Close() error {
	// Close closers in reverse.
	for i := len(r.closers) - 1; i >= 0; i-- {
		if err := r.closers[i].Close(); err != nil {
			return err
		}
	}
	return nil
}
