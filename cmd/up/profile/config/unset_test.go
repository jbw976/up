// Copyright 2025 Upbound Inc.
// All rights reserved

package config

import (
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestUnsetValidateInput(t *testing.T) {
	tf, _ := os.CreateTemp(t.TempDir(), "")

	type args struct {
		key  string
		file *os.File
	}
	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"KeyAndFile": {
			reason: "Supplying a key and file is invalid.",
			args: args{
				key:  "k",
				file: tf,
			},
			want: want{
				err: errors.New(errOnlyKVFileXOR),
			},
		},
		"KeyNoFile": {
			reason: "Supplying k and no file is valid.",
			args: args{
				key: "k",
			},
			want: want{},
		},
		"FileNoKeyValue": {
			reason: "Supplying no k and v, just file is valid.",
			args: args{
				file: tf,
			},
			want: want{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &unsetCmd{
				Key:  tc.args.key,
				File: tc.args.file,
			}

			err := c.validateInput()

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nValidateInput(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
