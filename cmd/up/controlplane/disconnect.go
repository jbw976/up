// Copyright 2025 Upbound Inc.
// All rights reserved

package controlplane

import (
	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

type disconnectCmd struct{}

// Run executes the get command.
func (c *disconnectCmd) Run() error {
	return errors.New("this command has been removed in favor of 'up ctx'. Use 'up ctx -' to return to the previous context")
}
