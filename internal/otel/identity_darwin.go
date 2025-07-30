//go:build darwin

// Copyright 2025 Upbound Inc.
// All rights reserved

package otel

import (
	"fmt"
	"os/exec"
	"strings"
)

// getPlatformID on macOS gets the IOPlatformUUID via the ioreg command.
func getPlatformID() (string, error) {
	cmd := exec.Command("ioreg", "-rd1", "-c", "IOPlatformExpertDevice")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to execute ioreg: %w", err)
	}

	for _, line := range strings.Split(string(output), "\n") {
		if strings.Contains(line, "IOPlatformUUID") {
			parts := strings.Split(line, "=")
			if len(parts) == 2 {
				return strings.Trim(parts[1], ` "`), nil
			}
		}
	}

	return "", fmt.Errorf("IOPlatformUUID not found")
}
