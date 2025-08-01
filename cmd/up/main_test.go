// Copyright 2025 Upbound Inc.
// All rights reserved

package main

import (
	"testing"

	"github.com/alecthomas/kong"
	"gotest.tools/v3/assert"
)

// TestKong ensures that the main command is a valid Kong structure. In
// particular, this validates that there are no duplicate flags or other nasty
// gotchas anywhere in the tree that would otherwise not show up until runtime.
func TestKong(t *testing.T) {
	_, err := kong.New(&cli{})
	assert.NilError(t, err)
}
