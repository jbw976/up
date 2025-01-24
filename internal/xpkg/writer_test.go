// Copyright 2025 Upbound Inc.
// All rights reserved

package xpkg

import (
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestNewMetaFile(t *testing.T) {
	fs := afero.NewMemMapFs()

	_ = afero.WriteFile(fs, filepath.Join("/previous", MetaFile), []byte{}, StreamFileMode)

	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		writer *Writer
		want   want
	}{
		"SuccessDirectoryDoesNotExist": {
			reason: "We should create the directory if it doesn't exist.",
			writer: NewFileWriter(
				WithRoot("/test"),
				WithFs(fs),
			),
		},
		"AlreadyExists": {
			reason: "We should return an error if a meta file already exists at the given location.",
			writer: NewFileWriter(
				WithRoot("/previous"),
				WithFs(fs),
			),
			want: want{
				err: errors.Errorf(errAlreadyExistsFmt, "crossplane.yaml"),
			},
		},
		"Successful": {
			reason: "We should nil if the file is successfully created.",
			writer: NewFileWriter(
				WithRoot("."),
				WithFs(fs),
			),
			want: want{
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.writer.NewMetaFile()

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nNewMetaFile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
