// Copyright 2025 Upbound Inc.
// All rights reserved

// Package kcl contains function for kcl embedded functions and tests helpers
package kcl

import "strings"

// FormatKclImportPath ensures unique aliases while converting paths.
func FormatKclImportPath(path string, existingAliases map[string]bool) (string, string) {
	// Find the position of "models" in the path
	modelsIndex := strings.Index(path, "models")
	if modelsIndex == -1 {
		return "", "" // Return empty values if "models" is not found
	}

	// Trim before "models" and replace slashes with dots
	importPath := strings.ReplaceAll(path[modelsIndex:], "/", ".")

	// Split path into components
	parts := strings.Split(importPath, ".")
	if len(parts) < 2 {
		return "", "" // Ensure there are enough components for alias creation
	}

	// Extract alias using the last two components (default behavior)
	alias := parts[len(parts)-2] + parts[len(parts)-1] // e.g., ec2v1beta1

	// If alias clashes, add more context (e.g., prefix with parent category)
	if existingAliases[alias] {
		for i := 3; i <= len(parts); i++ {
			alias = strings.Join(parts[len(parts)-i:], "")
			if !existingAliases[alias] {
				break
			}
		}
	}

	// Store the alias to prevent future clashes
	existingAliases[alias] = true

	return importPath, alias
}
