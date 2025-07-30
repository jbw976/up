//go:build !linux && !windows && !darwin

// Copyright 2025 Upbound Inc.
// All rights reserved

package otel

import "fmt"

// getPlatformID for other OSes returns an error to force the fallback method.
func getPlatformID() (string, error) {
	return "", fmt.Errorf("platform-specific ID not implemented for this OS")
}
