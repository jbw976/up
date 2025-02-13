// Copyright 2025 Upbound Inc.
// All rights reserved

package kube

import (
	"os"
	"path/filepath"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

func kubectxPrevCtxFile() (string, error) {
	home, err := os.UserHomeDir()
	if home == "" || err != nil {
		return "", errors.New("HOME or USERPROFILE environment variable not set")
	}
	return filepath.Join(home, ".kube", "kubectx"), nil
}

// ReadLastContext returns the saved previous context
// if the state file exists, otherwise returns "".
func ReadLastContext() (string, error) {
	path, err := kubectxPrevCtxFile()
	if err != nil {
		return "", err
	}
	bs, err := os.ReadFile(path) //nolint:gosec // it's ok
	if os.IsNotExist(err) {
		return "", nil
	}
	return string(bs), err
}

// WriteLastContext saves the specified value to the state file.
// It creates missing parent directories.
func WriteLastContext(value string) error {
	path, err := kubectxPrevCtxFile()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // it's ok
		return errors.Wrap(err, "failed to create parent directories")
	}
	return os.WriteFile(path, []byte(value), 0o644) //nolint:gosec // it's ok
}
