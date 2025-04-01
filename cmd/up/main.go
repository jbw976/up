// Copyright 2025 Upbound Inc.
// All rights reserved

package main

import (
	"context"
	"io"
	"os"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	"github.com/willabides/kongplete"

	"github.com/upbound/up/cmd/up/composition"
	"github.com/upbound/up/cmd/up/controlplane"
	"github.com/upbound/up/cmd/up/ctx"
	"github.com/upbound/up/cmd/up/dependency"
	"github.com/upbound/up/cmd/up/example"
	"github.com/upbound/up/cmd/up/function"
	"github.com/upbound/up/cmd/up/group"
	"github.com/upbound/up/cmd/up/login"
	"github.com/upbound/up/cmd/up/migration"
	"github.com/upbound/up/cmd/up/organization"
	"github.com/upbound/up/cmd/up/profile"
	"github.com/upbound/up/cmd/up/project"
	"github.com/upbound/up/cmd/up/query"
	"github.com/upbound/up/cmd/up/repository"
	"github.com/upbound/up/cmd/up/robot"
	"github.com/upbound/up/cmd/up/space"
	"github.com/upbound/up/cmd/up/team"
	"github.com/upbound/up/cmd/up/test"
	"github.com/upbound/up/cmd/up/trace"
	tviewtemplate "github.com/upbound/up/cmd/up/tview-template"
	"github.com/upbound/up/cmd/up/uxp"
	v "github.com/upbound/up/cmd/up/version"
	"github.com/upbound/up/cmd/up/xpkg"
	"github.com/upbound/up/cmd/up/xpls"
	"github.com/upbound/up/cmd/up/xrd"
	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/feature"
	"github.com/upbound/up/internal/upterm"

	// TODO(epk): Remove this once we upgrade kubernetes deps to 1.25
	// TODO(epk): Specifically, get rid of the k8s.io/client-go/client/auth/azure
	// and k8s.io/client-go/client/auth/gcp packages.
	// Embed Kubernetes client auth plugins.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

// AfterApply configures global settings before executing commands.
func (c *cli) AfterApply(ctx *kong.Context) error { //nolint:unparam // Kong requires an error return.
	if c.Quiet {
		ctx.Stdout, ctx.Stderr = io.Discard, io.Discard
	}
	ctx.BindTo(pterm.DefaultBasicText.WithWriter(ctx.Stdout), (*pterm.TextPrinter)(nil))
	// TODO(hasheddan): configure pretty print styling to match Upbound
	// branding.
	if !c.Pretty {
		// NOTE(hasheddan): enabling styling can make processing output with
		// other tooling difficult.
		pterm.DisableStyling()
	}

	printer := upterm.DefaultObjPrinter
	printer.DryRun = c.DryRun
	printer.Format = c.Format
	printer.Pretty = c.Pretty
	printer.Quiet = c.Quiet

	ctx.Bind(printer)
	ctx.BindTo(&printer, (*upterm.Printer)(nil))
	ctx.Bind(c.Quiet)
	return nil
}

// BeforeReset runs before all other hooks. Default maturity level is stable.
func (c *cli) BeforeReset(ctx *kong.Context, p *kong.Path) error {
	ctx.Bind(feature.Stable)
	// If no command is selected, we are emitting help and filter maturity.
	if ctx.Selected() == nil {
		return feature.HideMaturity(p, feature.Stable)
	}
	return nil
}

type cli struct {
	Format config.Format    `default:"default"           enum:"default,json,yaml" help:"Format for get/list commands. Can be: json, yaml, default" name:"format"`
	Quiet  config.QuietFlag `help:"Suppress all output." name:"quiet"             short:"q"`
	Pretty bool             `help:"Pretty print output." name:"pretty"`
	DryRun bool             `help:"dry-run output."      name:"dry-run"`

	// Manage Upbound Resources
	Organization organization.Cmd `aliases:"org"  cmd:""                           group:"Manage Upbound Resources"           help:"Interact with Upbound organizations."                                                         name:"organization"`
	Team         team.Cmd         `cmd:""         group:"Manage Upbound Resources" help:"Interact with teams."                name:"team"`
	Robot        robot.Cmd        `cmd:""         group:"Manage Upbound Resources" help:"Interact with robots."               name:"robot"`
	Repository   repository.Cmd   `aliases:"repo" cmd:""                           group:"Manage Upbound Resources"           help:"Interact with repositories."                                                                  name:"repository"`
	Space        space.Cmd        `cmd:""         group:"Manage Upbound Resources" help:"Interact with Spaces."`
	Group        group.Cmd        `cmd:""         group:"Manage Upbound Resources" help:"Interact with groups inside Spaces."`
	ControlPlane controlplane.Cmd `aliases:"ctp"  cmd:""                           group:"Manage Upbound Resources"           help:"Interact with control planes in the current context, both in the cloud and in a local Space." name:"controlplane"`
	UXP          uxp.Cmd          `cmd:""         group:"Manage Upbound Resources" help:"Interact with UXP."`

	// Develop with Crossplane
	Project     project.Cmd     `cmd:""        group:"Develop with Crossplane" help:"Manage Upbound development projects."`
	Example     example.Cmd     `cmd:""        group:"Develop with Crossplane" help:"Manage Claim(XRC) or Composite Resource(XR)."`
	Dependency  dependency.Cmd  `aliases:"dep" cmd:""                          group:"Develop with Crossplane"                                            help:"Manage configuration dependencies."`
	XRD         xrd.Cmd         `cmd:""        group:"Develop with Crossplane" help:"Manage XRDs from Composite Resources(XR) or Claims(XRC)."`
	Composition composition.Cmd `cmd:""        group:"Develop with Crossplane" help:"Manage Compositions."`
	Function    function.Cmd    `cmd:""        group:"Develop with Crossplane" help:"Manage Functions."`
	Test        test.Cmd        `cmd:""        group:"Develop with Crossplane" help:"Manage and run tests for projects."`
	XPKG        xpkg.Cmd        `cmd:""        group:"Develop with Crossplane" help:"Deprecated. Please migrate to up project or use the crossplane CLI." maturity:"deprecated"`
	XPLS        xpls.Cmd        `cmd:""        group:"Develop with Crossplane" help:"Start xpls language server."`

	// Configure up
	Completion kongplete.InstallCompletions `cmd:"" group:"Configure up" help:"Generate shell autocompletions"`
	Ctx        ctx.Cmd                      `cmd:"" group:"Configure up" help:"Select an Upbound kubeconfig context."`
	Help       helpCmd                      `cmd:"" group:"Configure up" help:"Show help."`
	License    licenseCmd                   `cmd:"" group:"Configure up" help:"Show license information."`
	Profile    profile.Cmd                  `cmd:"" group:"Configure up" help:"Manage configuration profiles."`
	Login      login.LoginCmd               `cmd:"" group:"Configure up" help:"Login to Upbound. Will attempt to launch a web browser by default. Use --username and --password flags for automations."`
	Logout     login.LogoutCmd              `cmd:"" group:"Configure up" help:"Logout of Upbound."`
	Version    v.Cmd                        `cmd:"" group:"Configure up" help:"Show current version."`

	Alpha alpha `cmd:"" group:"Alpha" help:"Alpha features. Commands may be removed in future releases."`
}

type helpCmd struct{}

func (h *helpCmd) Run(ctx *kong.Context) error {
	_, err := ctx.Parse([]string{"--help"})
	return err
}

// BeforeReset runs before all other hooks. If command has alpha as an ancestor,
// maturity level will be set to alpha.
func (a *alpha) BeforeReset(ctx *kong.Context) error { //nolint:unparam // Kong requires an error return.
	ctx.Bind(feature.Alpha)
	return nil
}

type alpha struct {
	// For now, we maintain compatibility for systems that may still use the alpha variant of `controlplane`.
	// This nudges users towards the stable variant when they attempt to emit help.
	ControlPlane  controlplane.Cmd  `aliases:"ctp" cmd:""                                                           help:"Interact with control planes in the current context, both in the cloud and in a local space." hidden:""        name:"controlplane"`
	Migration     migration.Cmd     `cmd:""        help:"Migrate control planes to Upbound Managed Control Planes." maturity:"alpha"`
	Trace         trace.Cmd         `cmd:""        help:"Trace a Crossplane resource."                              hidden:""                                                                                           maturity:"alpha"`
	TviewTemplate tviewtemplate.Cmd `cmd:""        help:"TView example."                                            hidden:""                                                                                           maturity:"alpha"`
	Query         query.QueryCmd    `cmd:""        help:"Query objects in one or many control planes."              hidden:""                                                                                           maturity:"alpha"`
	Get           query.GetCmd      `cmd:""        help:"Get objects in the current control plane."                 hidden:""                                                                                           maturity:"alpha"`
	Space         space.Cmd         `cmd:""        help:"Interact with Spaces."                                     maturity:"alpha"`
	Ctx           ctx.Cmd           `cmd:""        help:"Select an Upbound kubeconfig context."                     maturity:"alpha"`
	Xpkg          xpkg.Cmd          `cmd:""        help:"Manage Crossplane packages."                               hidden:""`
}

const helpDescription = `The Upbound CLI.

Please report issues and feature requests at https://github.com/upbound/upbound.`

func main() {
	c := cli{}

	parser := kong.Must(&c,
		kong.Name("up"),
		kong.Description(helpDescription),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact:             true,
			NoExpandSubcommands: true,
		}))

	kongplete.Complete(parser,
		kongplete.WithPredictor("orgs", organization.PredictOrgs()),
		kongplete.WithPredictor("ctps", controlplane.PredictControlPlanes()),
		kongplete.WithPredictor("repos", repository.PredictRepos()),
		kongplete.WithPredictor("robots", robot.PredictRobots()),
		kongplete.WithPredictor("teams", team.PredictTeams()),
		kongplete.WithPredictor("profiles", profile.PredictProfiles()),
		// TODO(sttts): add get and query
	)

	if len(os.Args) == 1 {
		_, err := parser.Parse([]string{"--help"})
		parser.FatalIfErrorf(err)
		return
	}

	kongCtx, err := parser.Parse(os.Args[1:])
	parser.FatalIfErrorf(err)
	kongCtx.BindTo(context.Background(), (*context.Context)(nil))
	kongCtx.FatalIfErrorf(kongCtx.Run())
}
