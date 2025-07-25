// Copyright 2025 Upbound Inc.
// All rights reserved

package config

// MockSource is a mock source.
type MockSource struct {
	InitializeFn   func() error
	GetConfigFn    func() (*Config, error)
	UpdateConfigFn func(*Config) error
}

// Initialize calls the underlying initialize function.
func (m *MockSource) Initialize() error {
	return m.InitializeFn()
}

// GetConfig calls the underlying get config function.
func (m *MockSource) GetConfig() (*Config, error) {
	return m.GetConfigFn()
}

// UpdateConfig calls the underlying update config function.
func (m *MockSource) UpdateConfig(c *Config) error {
	return m.UpdateConfigFn(c)
}
