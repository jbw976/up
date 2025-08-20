// Copyright 2025 Upbound Inc.
// All rights reserved

// Package stop provides the `up project stop` command.
package stop

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/pterm/pterm"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/upbound/up/internal/ctp"
	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/pkg/apis/project/v2alpha1"
)

// Cmd is the `up project stop` command.
type Cmd struct {
	ProjectFile       string `default:"upbound.yaml"                                                                                                                     help:"Path to project definition file."                short:"f"`
	ControlPlaneGroup string `help:"The control plane group that the control plane to use is contained in. This defaults to the group specified in the current context."`
	ControlPlaneName  string `help:"Name of the control plane to stop. Defaults to the project name."`
	SkipCheck         bool   `alias:"allow-production"                                                                                                                   help:"Allow stopping a non-development control plane." name:"skip-control-plane-check"`
	Force             bool   `help:"Do not ask for confirmation before stopping the control plane."`
	Local             bool   `help:"Find and stop a local dev control plane, even if Spaces is available."`

	proj *v2alpha1.Project
}

// AfterApply processes flags and sets defaults.
func (c *Cmd) AfterApply() error {
	// Read the project file.
	projFilePath, err := filepath.Abs(c.ProjectFile)
	if err != nil {
		return err
	}
	// The location of the project file defines the root of the project.
	projDirPath := filepath.Dir(projFilePath)
	// Construct a virtual filesystem that contains only the project. We'll do
	// all our operations inside this virtual FS.
	projFS := afero.NewBasePathFs(afero.NewOsFs(), projDirPath)

	prj, err := project.Parse(projFS, c.ProjectFile)
	if err != nil {
		return errors.New("this is not a project directory")
	}
	prj.Default()
	c.proj = prj

	return nil
}

// Run is the body of the command.
func (c *Cmd) Run(ctx context.Context, upCtx *upbound.Context) error {
	if c.ControlPlaneName == "" {
		c.ControlPlaneName = "up-" + c.proj.Name
	}

	ctp, found, err := ctp.FindDevControlPlane(ctx, upCtx,
		ctp.FindForceLocal(c.Local),
		ctp.FindSkipDevCheck(c.SkipCheck),
		ctp.FindWithSpacesGroup(c.ControlPlaneGroup),
		ctp.FindWithControlPlaneName(c.ControlPlaneName),
	)
	if err != nil {
		return errors.Wrap(err, "error while finding control plane")
	}

	if !found {
		pterm.Println("Development control plane not found.")
		return nil
	}

	if !c.Force {
		confirmMsg := fmt.Sprintf("Are you sure you want to destroy %s?", ctp.ShortDescription())
		proceed, err := pterm.DefaultInteractiveConfirm.Show(confirmMsg)
		if err != nil {
			return err
		}
		if !proceed {
			return errors.New("operation canceled")
		}
	}

	if err := ctp.Teardown(ctx, c.SkipCheck); err != nil {
		return errors.Wrap(err, "failed to stop control plane")
	}

	pterm.Println("Development control plane stopped.")
	return nil
}
