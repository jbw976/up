// Copyright 2025 Upbound Inc.
// All rights reserved

package space

import (
	"context"
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/upbound/up/internal/input"
	"github.com/upbound/up/internal/install/helm"
	"github.com/upbound/up/internal/registry"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

const (
	confirmStr      = "CONFIRMED"
	nsUpboundSystem = "upbound-system"
)

// destroyCmd uninstalls Upbound.
type destroyCmd struct {
	upbound.RequiresContext

	Registry registry.Flags `embed:""`

	Confirmed bool `help:"Bypass safety checks and destroy Spaces"                    name:"yes-really-delete-space-and-all-data" type:"bool"`
	Orphan    bool `help:"Remove Space components but retain Control Planes and data" name:"orphan"                               type:"bool"`
}

// AfterApply sets default values in command after assignment and validation.
func (c *destroyCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context, p upterm.Printer) error {
	kubeconfig, err := upCtx.GetKubeconfig()
	if err != nil {
		return err
	}

	// todo(redbackthomson): Migrate to using client.Client for standardization
	kClient, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return err
	}
	kongCtx.Bind(kClient)

	with := []helm.InstallerModifierFn{}
	if c.Orphan {
		with = append(with, helm.WithNoHooks())
	}

	mgr, err := helm.NewManager(kubeconfig,
		spacesChart,
		c.Registry.Repository,
		ns,
		with...,
	)
	if err != nil {
		return err
	}
	kongCtx.Bind(mgr)
	c.confirm(p)

	return nil
}

// Run executes the uninstall command.
func (c *destroyCmd) Run(ctx context.Context, kClient *kubernetes.Clientset, mgr *helm.Installer) error {
	if err := mgr.Uninstall(); err != nil {
		return err
	}

	// leave `upbound-system` namespace in place since there are secrets, configmaps, etc,
	// used by controlplanes
	if c.Orphan {
		return nil
	}

	return kClient.CoreV1().Namespaces().Delete(ctx, nsUpboundSystem, v1.DeleteOptions{})
}

// confirm prompts for confirmation and exits if the user declines.
func (c *destroyCmd) confirm(p upterm.Printer) {
	if c.Confirmed {
		return
	}
	if c.Orphan {
		p.Println()
		p.PrintInfo("Removing Space API components.")
		p.PrintInfo("Control Planes will continue to run and no data will be lost")
		p.Println()
	} else {
		p.Println()
		p.PrintError("******************** DESTRUCTIVE COMMAND ********************")
		p.PrintError("********************* DATA-LOSS WARNING *********************")
		p.Println()
		p.PrintWarning("Destroying Spaces is a destructive command that will destroy data and orphan resources.")
		p.PrintWarning("Before proceeding ensure that Managed Resources in Control Planes have been deleted.")
		p.PrintWarning("All Spaces components including Control Planes will be destroyed.")
		p.Println()
		p.PrintWarning("If you want to retain data, abort and run 'up space destroy --orphan'")
		p.Println()
	}

	prompter := input.NewPrompter()
	in, err := prompter.Prompt(fmt.Sprintf("To proceed, type: %q", confirmStr), false)
	if err != nil {
		p.PrintError("error getting user confirmation:", err)
		os.Exit(1)
	}
	if in != confirmStr {
		p.PrintError("Destruction was not confirmed")
		os.Exit(10)
	}
}
