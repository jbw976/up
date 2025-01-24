// Copyright 2025 Upbound Inc.
// All rights reserved

// Package ndjson contains an ndjson package parser.
package ndjson

import (
	"bufio"
	"bytes"
	"io"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// LineReader represents a reader that reads from the underlying reader
// line by line, separated by '\n'.
type LineReader struct {
	reader *bufio.Reader
}

// NewReader returns a new reader, using the underlying io.Reader
// as input.
func NewReader(r *bufio.Reader) *LineReader {
	return &LineReader{reader: r}
}

// Read returns a single line (with '\n' ended) from the underlying reader.
// An error is returned iff there is an error with the underlying reader.
func (r *LineReader) Read() ([]byte, error) {
	for {
		line, err := r.reader.ReadBytes('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}

		// skip blank lines
		if len(line) != 0 && !bytes.Equal(line, []byte{'\n'}) {
			return line, nil
		}

		// EOF seen and there's nothing left in the reader, return EOF.
		if errors.Is(err, io.EOF) {
			return nil, err
		}
	}
}
