// Copyright 2025 Upbound Inc.
// All rights reserved

package test

import (
	"fmt"
	"strings"

	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/runtime"
)

func writeToFile(fs afero.Fs, resources []runtime.RawExtension, filename string) (string, error) {
	if len(resources) == 0 {
		return "", nil
	}

	// Define file path
	filePath := fmt.Sprintf("/resources/%s.yaml", filename)

	// Ensure directory exists
	if err := fs.MkdirAll("/resources", 0o755); err != nil {
		return "", err
	}

	// Open file for writing (Create or Truncate existing)
	file, err := fs.Create(filePath)
	if err != nil {
		return "", err
	}

	var content []byte
	for _, res := range resources {
		trimmed := strings.TrimSpace(string(res.Raw)) // Trim leading/trailing whitespace
		content = append(content, []byte(trimmed)...)
		content = append(content, []byte("\n---\n")...) // Ensure correct separator format
	}

	// Write content to file
	if _, err := file.Write(content); err != nil {
		return "", err
	}

	return filePath, nil
}

// writeContextToFile writes a single context value to a file without YAML document separators.
func writeContextToFile(fs afero.Fs, value runtime.RawExtension, filename string) (string, error) {
	if len(value.Raw) == 0 {
		return "", nil
	}

	// Replace path separators and other problematic characters with safe alternatives
	// This preserves uniqueness while making the filename safe for filesystems
	safeFilename := strings.ReplaceAll(filename, "/", "-")

	// Define file path
	filePath := fmt.Sprintf("/resources/%s.json", safeFilename)

	// Ensure directory exists
	if err := fs.MkdirAll("/resources", 0o755); err != nil {
		return "", err
	}

	// Write the raw JSON directly without any separators
	if err := afero.WriteFile(fs, filePath, value.Raw, 0o644); err != nil {
		return "", err
	}

	return filePath, nil
}
