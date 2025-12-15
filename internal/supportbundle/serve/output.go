// Copyright 2025 Upbound Inc.
// All rights reserved

// Package serve provides functionality to serve support bundle contents
// through a local Kubernetes API server for interactive exploration.
package serve

import (
	"io"

	"github.com/mesosphere/dkp-cli-runtime/core/output"
)

// outputAdapter adapts to troubleshoot-live's output interface.
// When debug is enabled, messages are printed using the debugf function.
// The main idea here is to drop all logging messages for a clean CLI output
// and only print logging (which we don't control) when debug is enabled.
type outputAdapter struct {
	debug  bool
	debugf func(format string, args ...any)
}

func (o *outputAdapter) StartOperation(msg string)                          { o.logf(msg) }
func (o *outputAdapter) StartOperationWithProgress(_ *output.ProgressGauge) {}
func (o *outputAdapter) EndOperation(_ bool)                                {}
func (o *outputAdapter) EndOperationWithStatus(_ output.EndOperationStatus) {}
func (o *outputAdapter) Info(s string)                                      { o.logf(s) }
func (o *outputAdapter) Infof(format string, args ...any)                   { o.logf(format, args...) }
func (o *outputAdapter) Result(result string)                               { o.logf(result) }
func (o *outputAdapter) Warn(msg string)                                    { o.logf("WARNING: %s", msg) }
func (o *outputAdapter) Warnf(format string, args ...any)                   { o.logf("WARNING: "+format, args...) }
func (o *outputAdapter) InfoWriter() io.Writer                              { return o.writer() }
func (o *outputAdapter) ResultWriter() io.Writer                            { return o.writer() }
func (o *outputAdapter) WarnWriter() io.Writer                              { return o.writer() }
func (o *outputAdapter) ErrorWriter() io.Writer                             { return o.writer() }
func (o *outputAdapter) WithValues(_ ...any) output.Output                  { return o }
func (o *outputAdapter) V(_ int) output.Output                              { return o }

// Error logs an error message with optional error value.
func (o *outputAdapter) Error(err error, msg string) {
	if err != nil {
		o.logf("ERROR: %s: %v", msg, err)
	} else {
		o.logf("ERROR: %s", msg)
	}
}

// Errorf logs a formatted error message with optional error value.
func (o *outputAdapter) Errorf(err error, format string, args ...any) {
	if err != nil {
		o.logf("ERROR: "+format+": %v", append(args, err)...)
	} else {
		o.logf("ERROR: "+format, args...)
	}
}

func (o *outputAdapter) logf(format string, args ...any) {
	if o.debug && o.debugf != nil {
		o.debugf(format, args...)
	}
}

func (o *outputAdapter) writer() io.Writer {
	if o.debug && o.debugf != nil {
		return &debugWriter{debugf: o.debugf}
	}
	return io.Discard
}

// debugWriter writes to the debug function.
type debugWriter struct {
	debugf func(format string, args ...any)
}

func (d *debugWriter) Write(p []byte) (int, error) {
	d.debugf("%s", p)
	return len(p), nil
}
