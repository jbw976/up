// Copyright 2025 Upbound Inc.
// All rights reserved

package aws

import (
	"compress/gzip"
	"context"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/upbound/up/internal/usage/encoding/json"
	"github.com/upbound/up/internal/usage/event"
	"github.com/upbound/up/internal/usage/event/reader"
	"github.com/upbound/up/internal/usage/model"
)

var _ event.Reader = &ListObjectsV2InputEventReader{}

// ListObjectsV2InputEventReader reads usage events from a
// *s3.ListObjectsV2Input.
type ListObjectsV2InputEventReader struct {
	Client             *s3.Client
	Bucket             string
	ListObjectsV2Input *s3.ListObjectsV2Input
	reader             *reader.MultiReader
}

func (r *ListObjectsV2InputEventReader) Read(ctx context.Context) (model.MXPGVKEvent, error) {
	if r.reader == nil {
		readers := []event.Reader{}
		paginator := s3.NewListObjectsV2Paginator(r.Client, r.ListObjectsV2Input)
		for paginator.HasMorePages() {
			page, err := paginator.NextPage(ctx)
			if err != nil {
				return model.MXPGVKEvent{}, err
			}
			for _, obj := range page.Contents {
				readers = append(readers, &GetObjectInputEventReader{
					Client: r.Client,
					GetObjectInput: &s3.GetObjectInput{
						Bucket: aws.String(r.Bucket),
						Key:    obj.Key,
					},
				})
			}
		}
		r.reader = &reader.MultiReader{Readers: readers}
	}
	return r.reader.Read(ctx)
}

// Close closes the underlying reader.
func (r *ListObjectsV2InputEventReader) Close() error {
	if r.reader == nil {
		return nil
	}
	return r.reader.Close()
}

var _ event.Reader = &GetObjectInputEventReader{}

// GetObjectInputEventReader reads usage events from a *s3.GetObjectInput.
type GetObjectInputEventReader struct {
	Client         *s3.Client
	GetObjectInput *s3.GetObjectInput
	decoder        *json.MXPGVKEventDecoder
	closers        []io.Closer
}

func (r *GetObjectInputEventReader) Read(ctx context.Context) (model.MXPGVKEvent, error) {
	if r.decoder == nil {
		// TODO(branden): Use s3manager.Downloader for streaming and concurrent
		// downloads.
		resp, err := r.Client.GetObject(ctx, r.GetObjectInput)
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
		return model.MXPGVKEvent{}, event.ErrEOF
	}
	return r.decoder.Decode()
}

// Close closes all underlying resources in reverse order.
func (r *GetObjectInputEventReader) Close() error {
	// Close closers in reverse.
	for i := len(r.closers) - 1; i >= 0; i-- {
		if err := r.closers[i].Close(); err != nil {
			return err
		}
	}
	return nil
}
