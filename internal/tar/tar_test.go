// Copyright 2025 Upbound Inc.
// All rights reserved

package tar

import (
	"archive/tar"
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/spf13/afero"
)

func TestExtractTo(t *testing.T) {
	type args struct {
		tarData []byte
		source  string
		dest    string
	}
	type want struct {
		err   error
		files map[string]string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ExtractSingleFile": {
			reason: "Should extract a single file to destination path.",
			args: args{
				tarData: createTarWithFiles(map[string]string{
					"report/data.csv": "line1\nline2\n",
					"other/file.txt":  "ignored",
				}),
				source: "report/data.csv",
				dest:   "extracted/data.csv",
			},
			want: want{
				files: map[string]string{
					"extracted/data.csv": "line1\nline2\n",
				},
			},
		},
		"ExtractDirectory": {
			reason: "Should extract a directory and its contents to destination path.",
			args: args{
				tarData: createTarWithFiles(map[string]string{
					"report/data.csv":     "csv data",
					"report/meta.json":    "json data",
					"report/sub/file.txt": "sub file",
					"other/ignored.txt":   "ignored",
				}),
				source: "report",
				dest:   "extracted",
			},
			want: want{
				files: map[string]string{
					"extracted/data.csv":     "csv data",
					"extracted/meta.json":    "json data",
					"extracted/sub/file.txt": "sub file",
				},
			},
		},
		"ExtractNonExistentFile": {
			reason: "Should return ErrNotFound when source file does not exist.",
			args: args{
				tarData: createTarWithFiles(map[string]string{
					"report/data.csv": "data",
				}),
				source: "nonexistent/file.txt",
				dest:   "extracted/file.txt",
			},
			want: want{
				err:   ErrNotFound,
				files: map[string]string{},
			},
		},
		"ExtractEmptyTar": {
			reason: "Should return ErrNotFound when tar archive is empty.",
			args: args{
				tarData: createEmptyTar(),
				source:  "any/file.txt",
				dest:    "extracted/file.txt",
			},
			want: want{
				err:   ErrNotFound,
				files: map[string]string{},
			},
		},
		"ExtractWithSymlink": {
			reason: "Should return ErrUnsupportedFileType when tar contains symlinks.",
			args: args{
				tarData: createTarWithSymlink("report/link", "target"),
				source:  "report/link",
				dest:    "extracted/link",
			},
			want: want{
				err:   ErrUnsupportedFileType,
				files: map[string]string{},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			r := bytes.NewReader(tc.args.tarData)

			err := ExtractTo(r, fs, tc.args.source, tc.args.dest)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nExtractTo(...): -want err, +got err\n%s", name, diff)
			}

			files := mapFromFS(fs)
			if diff := cmp.Diff(tc.want.files, files); diff != "" {
				t.Errorf("%s\nExtractTo(...): -want, +got\n%s", name, diff)
			}
		})
	}
}

func TestExtractAll(t *testing.T) {
	type args struct {
		tarData []byte
	}
	type want struct {
		err   error
		files map[string]string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ExtractAllFiles": {
			reason: "Should extract all files and directories from tar archive.",
			args: args{
				tarData: createTarWithFiles(map[string]string{
					"report/data.csv":     "csv data",
					"report/meta.json":    "json data",
					"report/sub/file.txt": "sub file",
					"other/file.txt":      "other file",
				}),
			},
			want: want{
				files: map[string]string{
					"report/data.csv":     "csv data",
					"report/meta.json":    "json data",
					"report/sub/file.txt": "sub file",
					"other/file.txt":      "other file",
				},
			},
		},
		"ExtractEmptyTar": {
			reason: "Should successfully handle empty tar archive.",
			args: args{
				tarData: createEmptyTar(),
			},
			want: want{
				files: map[string]string{},
			},
		},
		"ExtractWithSymlink": {
			reason: "Should return ErrUnsupportedFileType when tar contains symlinks.",
			args: args{
				tarData: createTarWithSymlink("report/link", "target"),
			},
			want: want{
				err:   ErrUnsupportedFileType,
				files: map[string]string{},
			},
		},
		"ExtractSingleFile": {
			reason: "Should extract single file in tar archive.",
			args: args{
				tarData: createTarWithFiles(map[string]string{
					"single.txt": "single file content",
				}),
			},
			want: want{
				files: map[string]string{
					"single.txt": "single file content",
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			r := bytes.NewReader(tc.args.tarData)

			err := ExtractAll(r, fs)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nExtractAll(...): -want err, +got err\n%s", name, diff)
			}

			files := mapFromFS(fs)
			if diff := cmp.Diff(tc.want.files, files); diff != "" {
				t.Errorf("%s\nExtractAll(...): -want, +got\n%s", name, diff)
			}
		})
	}
}

func mapFromFS(fs afero.Fs) map[string]string {
	files := make(map[string]string)

	err := afero.Walk(fs, ".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if path == "." {
			return nil
		}

		content, err := afero.ReadFile(fs, path)
		if err != nil {
			return err
		}
		files[path] = string(content)
		return nil
	})
	if err != nil {
		panic(err)
	}

	return files
}

func createTarWithFiles(files map[string]string) []byte {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	// Create directories first
	dirs := make(map[string]bool)
	for path := range files {
		parts := strings.Split(path, "/")
		for i := range len(parts) - 1 {
			dirPath := strings.Join(parts[:i+1], "/")
			if !dirs[dirPath] {
				dirs[dirPath] = true
				hdr := &tar.Header{
					Name:     dirPath,
					Mode:     0o750,
					Typeflag: tar.TypeDir,
				}
				tw.WriteHeader(hdr)
			}
		}
	}

	// Add files
	for path, content := range files {
		hdr := &tar.Header{
			Name:     path,
			Mode:     0o640,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
		}
		tw.WriteHeader(hdr)
		tw.Write([]byte(content))
	}

	if err := tw.Close(); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func createEmptyTar() []byte {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.Close(); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func createTarWithSymlink(name, target string) []byte {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	hdr := &tar.Header{
		Name:     name,
		Linkname: target,
		Typeflag: tar.TypeSymlink,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		panic(err)
	}

	if err := tw.Close(); err != nil {
		panic(err)
	}
	return buf.Bytes()
}
