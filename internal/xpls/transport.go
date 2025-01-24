// Copyright 2025 Upbound Inc.
// All rights reserved

package xpls

import "os"

// StdRWC is a readwritecloser on stdio, which can be used as a JSON-RPC
// transport.
type StdRWC struct{}

// Read reads from stdin.
func (StdRWC) Read(p []byte) (int, error) {
	return os.Stdin.Read(p)
}

// Write writes to stdout.
func (StdRWC) Write(p []byte) (int, error) {
	return os.Stdout.Write(p)
}

// Close first closes stdin, then, if successful, closes stdout.
func (StdRWC) Close() error {
	if err := os.Stdin.Close(); err != nil {
		return err
	}
	return os.Stdout.Close()
}
