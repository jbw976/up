// Copyright 2025 Upbound Inc.
// All rights reserved

package otel

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"sort"
	"strings"
)

// machineID generates a unique identifier for the machine.
// It uses OS-specific methods and falls back to MAC addresses.
// The result is a SHA-256 hash of the collected identifier.
func machineID() (string, error) {
	// getPlatformID() is implemented in platform-specific files.
	id, err := getPlatformID()

	// If the platform-specific method fails, fall back to MAC addresses.
	if err != nil || id == "" {
		id, err = getMacAddresses()
		if err != nil {
			return "", fmt.Errorf("all methods failed to generate a machine ID: %w", err)
		}
	}

	// Hash the identifier to ensure it's a fixed length and anonymous.
	hash := sha256.Sum256([]byte(id))
	return hex.EncodeToString(hash[:]), nil
}

// getMacAddresses collects and concatenates all hardware MAC addresses.
func getMacAddresses() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("failed to get network interfaces: %w", err)
	}

	var macs []string
	for _, iface := range interfaces {
		// We only want hardware interfaces, not virtual ones
		if iface.Flags&net.FlagLoopback == 0 && iface.HardwareAddr != nil {
			macs = append(macs, iface.HardwareAddr.String())
		}
	}

	if len(macs) == 0 {
		return "", fmt.Errorf("no suitable network interfaces found")
	}

	// Sort to ensure consistent order
	sort.Strings(macs)
	return strings.Join(macs, ""), nil
}

func getIdentity() string {
	identity, err := machineID()
	if err != nil {
		return "unknown"
	}
	return identity
}
