// Copyright 2025 Upbound Inc.
// All rights reserved

package main

import "github.com/upbound/up/internal/upterm"

// licenseCmd prints license information for using Up.
type licenseCmd struct{}

// Run executes the license command.
func (c *licenseCmd) Run(p upterm.Printer) error {
	p.Println("By using Up, you are accepting to comply with terms and conditions in https://licenses.upbound.io/upbound-software-license.html")
	return nil
}
