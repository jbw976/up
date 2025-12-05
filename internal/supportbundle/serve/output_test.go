// Copyright 2025 Upbound Inc.
// All rights reserved

package serve

import (
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
)

func TestOutputAdapter_Logf(t *testing.T) {
	tests := map[string]struct {
		debug  bool
		format string
		args   []any
		want   string
	}{
		"logs message when debug enabled": {
			debug:  true,
			format: "hello %s",
			args:   []any{"world"},
			want:   "hello %s",
		},
		"does not log when debug disabled": {
			debug:  false,
			format: "hello %s",
			args:   []any{"world"},
			want:   "",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var got string
			o := &outputAdapter{
				debug:  tt.debug,
				debugf: func(format string, _ ...any) { got = format },
			}

			o.logf(tt.format, tt.args...)

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("logf(): -want, +got\n%s", diff)
			}
		})
	}
}

func TestOutputAdapter_Writer(t *testing.T) {
	tests := map[string]struct {
		debug       bool
		wantDiscard bool
	}{
		"returns debugWriter when debug enabled": {
			debug:       true,
			wantDiscard: false,
		},
		"returns discard when debug disabled": {
			debug:       false,
			wantDiscard: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			o := &outputAdapter{
				debug:  tt.debug,
				debugf: func(_ string, _ ...any) {},
			}

			w := o.writer()
			gotDiscard := w == io.Discard

			if diff := cmp.Diff(tt.wantDiscard, gotDiscard); diff != "" {
				t.Errorf("writer(): -want, +got\n%s", diff)
			}
		})
	}
}

func TestOutputAdapter_Info(t *testing.T) {
	tests := map[string]struct {
		msg  string
		want string
	}{
		"logs message": {
			msg:  "test message",
			want: "test message",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var got string
			o := &outputAdapter{
				debug:  true,
				debugf: func(format string, _ ...any) { got = format },
			}

			o.Info(tt.msg)

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Info(): -want, +got\n%s", diff)
			}
		})
	}
}

func TestOutputAdapter_Warn(t *testing.T) {
	tests := map[string]struct {
		msg        string
		wantFormat string
	}{
		"prefixes with WARNING": {
			msg:        "something happened",
			wantFormat: "WARNING: %s",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var got string
			o := &outputAdapter{
				debug:  true,
				debugf: func(format string, _ ...any) { got = format },
			}

			o.Warn(tt.msg)

			if diff := cmp.Diff(tt.wantFormat, got); diff != "" {
				t.Errorf("Warn(): -want, +got\n%s", diff)
			}
		})
	}
}

func TestOutputAdapter_Error(t *testing.T) {
	tests := map[string]struct {
		err        error
		msg        string
		wantFormat string
	}{
		"with error includes error": {
			err:        errors.New("boom"),
			msg:        "operation failed",
			wantFormat: "ERROR: %s: %v",
		},
		"without error omits error": {
			err:        nil,
			msg:        "operation failed",
			wantFormat: "ERROR: %s",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var got string
			o := &outputAdapter{
				debug:  true,
				debugf: func(format string, _ ...any) { got = format },
			}

			o.Error(tt.err, tt.msg)

			if diff := cmp.Diff(tt.wantFormat, got); diff != "" {
				t.Errorf("Error(): -want, +got\n%s", diff)
			}
		})
	}
}

func TestOutputAdapter_Errorf(t *testing.T) {
	tests := map[string]struct {
		err        error
		format     string
		args       []any
		wantFormat string
	}{
		"with error appends error": {
			err:        errors.New("boom"),
			format:     "failed to process %s",
			args:       []any{"item"},
			wantFormat: "ERROR: failed to process %s: %v",
		},
		"without error omits error": {
			err:        nil,
			format:     "failed to process %s",
			args:       []any{"item"},
			wantFormat: "ERROR: failed to process %s",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var got string
			o := &outputAdapter{
				debug:  true,
				debugf: func(format string, _ ...any) { got = format },
			}

			o.Errorf(tt.err, tt.format, tt.args...)

			if diff := cmp.Diff(tt.wantFormat, got); diff != "" {
				t.Errorf("Errorf(): -want, +got\n%s", diff)
			}
		})
	}
}

func TestDebugWriter_Write(t *testing.T) {
	tests := map[string]struct {
		input string
		wantN int
	}{
		"returns byte count": {
			input: "hello world",
			wantN: 11,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			w := &debugWriter{
				debugf: func(_ string, _ ...any) {},
			}

			n, err := w.Write([]byte(tt.input))
			if err != nil {
				t.Errorf("Write() error: %v", err)
			}
			if diff := cmp.Diff(tt.wantN, n); diff != "" {
				t.Errorf("Write() n: -want, +got\n%s", diff)
			}
		})
	}
}
