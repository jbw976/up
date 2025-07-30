//go:build windows

// Copyright 2025 Upbound Inc.
// All rights reserved

package otel

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

// getPlatformID on Windows reads the unique MachineGuid from the registry.
func getPlatformID() (string, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return "", fmt.Errorf("failed to open registry key: %w", err)
	}
	defer k.Close()

	s, _, err := k.GetStringValue("MachineGuid")
	if err != nil {
		return "", fmt.Errorf("failed to read MachineGuid: %w", err)
	}
	return s, nil
}
