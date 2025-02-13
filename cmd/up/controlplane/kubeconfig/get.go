// Copyright 2025 Upbound Inc.
// All rights reserved

package kubeconfig

import (
	"context"
	"fmt"
)

// getCmd gets kubeconfig data for an Upbound control plane.
type getCmd struct {
	ConnectionSecretCmd

	File    string `help:"File to merge control plane kubeconfig into or to create. By default it is merged into the user's default kubeconfig. Use '-' to print it to stdout.'" short:"f" type:"path"`
	Context string `help:"Context to use in the kubeconfig."                                                                                                                     short:"c"`
}

// Run executes the get command.
func (c *getCmd) Run(ctx context.Context) error {
	return fmt.Errorf("this command has been removed in favor of 'up ctx <organization>/<space name>/%s/%s'", c.Group, c.Name)
}
