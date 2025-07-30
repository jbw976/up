//go:build linux

// Copyright 2025 Upbound Inc.
// All rights reserved

package otel

import (
	"fmt"
	"os"
	"strings"
)

// getPlatformID on Linux reads the unique machine-id.
func getPlatformID() (string, error) {
	data, err := os.ReadFile("/etc/machine-id")
	if err != nil {
		// Fallback for some systems that use a different path.
		data, err = os.ReadFile("/var/lib/dbus/machine-id")
		if err != nil {
			return "", fmt.Errorf("failed to read linux machine-id files: %w", err)
		}
	}
	return strings.TrimSpace(string(data)), nil
}
