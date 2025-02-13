// Copyright 2025 Upbound Inc.
// All rights reserved

package template

import (
	"context"
	"fmt"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type Cmd struct{}

func (c *Cmd) Help() string {
	return `
Usage:
    tview-template [options]

The 'tview-template' brings happiness.`
}

// BeforeApply sets default values for the delete command, before assignment and validation.
func (c *Cmd) BeforeApply() error {
	return nil
}

func (c *Cmd) Run(ctx context.Context) error {
	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to get kubeconfig: %w", err)
	}
	client, err := rest.TransportFor(cfg)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	app := NewApp("upbound tview-template", client, cfg.Host)
	return app.Run(ctx)
}
