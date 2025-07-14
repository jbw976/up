// Copyright 2025 Upbound Inc.
// All rights reserved

package controlplane

import (
	"testing"

	"github.com/alecthomas/kong"
	"gotest.tools/v3/assert"

	"github.com/upbound/up/cmd/up/controlplane/requires"
)

// TestRequirements ensures that all subcommands have a context requirement and
// will thus end up with a valid kube client.
func TestRequirements(t *testing.T) {
	kongCmd, err := kong.New(&Cmd{})
	assert.NilError(t, err)
	ctpCmd := kongCmd.Model.Node

	for _, child := range ctpCmd.Children {
		if _, ok := child.Target.Interface().(requires.Checker); !ok {
			t.Errorf("subcommand %s does not have a context requirement", child.Name)
		}
	}
}
