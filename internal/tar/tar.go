// Copyright 2025 Upbound Inc.
// All rights reserved

// Package tar provides utilities for interacting with tar files.
package tar

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
)

var (
	// ErrNotFound is returned if a file to be extracted is not found.
	ErrNotFound = errors.New("not found")
	// ErrUnsupportedFileType is returned if a file to be extracted is of any
	// type except regular or directory.
	ErrUnsupportedFileType = errors.New("unsupported file type")
)

// ExtractTo extracts the file at source in tarFile to dest in fs. If source is
// a directory, its contents are also extracted. Returns ErrUnsupportedFileType
// if source is or contains anything except directories and regular files.
// Returns ErrNotFound if source is not found.
func ExtractTo(tarFile io.Reader, fs afero.Fs, source, dest string) error {
	tr := tar.NewReader(tarFile)
	found := false

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.Wrap(err, "failed to read entry")
		}

		// Check if this entry matches our source or is within our source
		// directory.
		if hdr.Name == source || strings.HasPrefix(hdr.Name, source+"/") {
			found = true

			// Replace source with dest in destination path.
			destPath := hdr.Name
			if hdr.Name == source {
				destPath = dest
			} else if strings.HasPrefix(hdr.Name, source+"/") {
				relPath := strings.TrimPrefix(hdr.Name, source+"/")
				destPath = filepath.Join(dest, relPath)
			}

			if err := extractEntry(tr, fs, hdr, destPath); err != nil {
				return errors.Wrapf(err, "failed to extract %q to %q", hdr.Name, destPath)
			}
		}
	}

	if !found {
		return ErrNotFound
	}

	return nil
}

// ExtractAll extracts all files in tarFile to fs. Returns
// ErrUnsupportedFileType if tarFile contains anything except directories and
// regular files.
func ExtractAll(tarFile io.Reader, fs afero.Fs) error {
	tr := tar.NewReader(tarFile)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.Wrap(err, "failed to read tar entry")
		}

		if err := extractEntry(tr, fs, hdr, hdr.Name); err != nil {
			return errors.Wrapf(err, "failed to extract %q", hdr.Name)
		}
	}

	return nil
}

// extractEntry extracts a single entry from tr to fs. Returns an error if the
// entry is or contains anything except directories and regular files.
func extractEntry(tr *tar.Reader, fs afero.Fs, hdr *tar.Header, dest string) error {
	switch hdr.Typeflag {
	case tar.TypeDir:
		err := fs.MkdirAll(dest, hdr.FileInfo().Mode())
		return errors.Wrap(err, "failed to create directory")
	case tar.TypeReg:
		if err := fs.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
			return errors.Wrap(err, "failed to create parent directories")
		}

		file, err := fs.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, hdr.FileInfo().Mode())
		if err != nil {
			return errors.Wrap(err, "failed to create file")
		}

		_, err = io.Copy(file, tr)
		if err != nil {
			file.Close() //nolint:errcheck // Already handling an error
			return errors.Wrap(err, "failed to copy file content")
		}
		return errors.Wrap(file.Close(), "failed to close file")
	default:
		return ErrUnsupportedFileType
	}
}
