// Copyright 2025 Upbound Inc.
// All rights reserved

package controlplane

import (
	"context"
	"fmt"

	"github.com/upbound/up/cmd/up/controlplane/kubeconfig"
)

type connectCmd struct {
	kubeconfig.ConnectionSecretCmd `cmd:""`
}

// Run executes the get command.
func (c *connectCmd) Run(ctx context.Context) error {
	return fmt.Errorf("this command has been removed in favor of 'up ctx <organization>/<space name>/%s/%s'", c.Group, c.Name)
}
