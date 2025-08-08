// Copyright 2025 Upbound Inc.
// All rights reserved

// Package main is the up CLI.
package main

import (
	"context"
	"io"
	"os"
	"sync"
	"time"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	"github.com/willabides/kongplete"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/term"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/cmd/up/composition"
	configcmd "github.com/upbound/up/cmd/up/config"
	"github.com/upbound/up/cmd/up/controlplane"
	"github.com/upbound/up/cmd/up/ctx"
	"github.com/upbound/up/cmd/up/dependency"
	"github.com/upbound/up/cmd/up/example"
	"github.com/upbound/up/cmd/up/function"
	"github.com/upbound/up/cmd/up/group"
	"github.com/upbound/up/cmd/up/login"
	"github.com/upbound/up/cmd/up/migration"
	"github.com/upbound/up/cmd/up/operation"
	"github.com/upbound/up/cmd/up/organization"
	"github.com/upbound/up/cmd/up/profile"
	"github.com/upbound/up/cmd/up/project"
	"github.com/upbound/up/cmd/up/query"
	"github.com/upbound/up/cmd/up/repository"
	"github.com/upbound/up/cmd/up/robot"
	"github.com/upbound/up/cmd/up/runner"
	"github.com/upbound/up/cmd/up/space"
	"github.com/upbound/up/cmd/up/team"
	"github.com/upbound/up/cmd/up/test"
	"github.com/upbound/up/cmd/up/token"
	tracecmd "github.com/upbound/up/cmd/up/trace"
	"github.com/upbound/up/cmd/up/uxp"
	v "github.com/upbound/up/cmd/up/version"
	"github.com/upbound/up/cmd/up/xpkg"
	"github.com/upbound/up/cmd/up/xpls"
	"github.com/upbound/up/cmd/up/xrd"
	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/feature"
	"github.com/upbound/up/internal/otel"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"

	_ "embed"
	// TODO(epk): Remove this once we upgrade kubernetes deps to 1.25
	// TODO(epk): Specifically, get rid of the k8s.io/client-go/client/auth/azure
	// and k8s.io/client-go/client/auth/gcp packages.
	// Embed Kubernetes client auth plugins.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

// Global OTEL client and span for graceful shutdown.
//
//nolint:gochecknoglobals // We need this for graceful shutdown.
var (
	globalOTELClient  *otel.Client
	globalCommandSpan trace.Span
)

// AfterApply configures global settings before executing commands.
func (c *cli) AfterApply(kongCtx *kong.Context) error {
	if c.Quiet {
		kongCtx.Stdout, kongCtx.Stderr = io.Discard, io.Discard
	}
	kongCtx.BindTo(pterm.DefaultBasicText.WithWriter(kongCtx.Stdout), (*pterm.TextPrinter)(nil))

	var pretty bool
	if c.Pretty != nil {
		pretty = *c.Pretty
	} else {
		pretty = term.IsTerminal(int(os.Stdout.Fd()))
	}

	pterm.EnableStyling()
	if !pretty {
		pterm.DisableStyling()
	}

	printer := upterm.DefaultObjPrinter
	printer.DryRun = c.DryRun
	printer.Format = c.Format
	printer.Pretty = pretty
	printer.Quiet = c.Quiet

	kongCtx.Bind(printer)
	kongCtx.BindTo(&printer, (*upterm.Printer)(nil))
	kongCtx.Bind(c.Quiet)
	kongCtx.BindTo(&RootCommandRunner{}, (*runner.CommandRunner)(nil))

	// Initialize and bind OpenTelemetry client
	f := sync.OnceFunc(func() {
		if err := c.initOTEL(kongCtx); err != nil {
			pterm.Error.Printfln("Error initializing telemetry: %v", err)
			os.Exit(1)
		}
	})
	f()

	globalCommandSpan = createCommandSpans(kongCtx)

	// Bind the span for commands to use.
	//
	// Commands can add custom telemetry attributes by:
	// 1. Run(...., span trace.Span) - creating child spans
	// 2. Adding events: span.AddEvent("event-name", trace.WithAttributes(...)). See version.go for example.
	kongCtx.BindTo(globalCommandSpan, (*trace.Span)(nil))

	// If the command (or any of its parents) requires an up context, construct
	// it and bind it to the kongCtx.
	for current := kongCtx.Selected(); current != nil; current = current.Parent {
		if req, ok := current.Target.Interface().(upbound.ContextRequirer); ok {
			upCtx, err := req.GetUpboundContext()
			if err != nil {
				return errors.Wrap(err, "failed to construct upbound context")
			}
			kongCtx.Bind(upCtx)

			// Set a span attribute indicating whether the user is logged in.
			globalCommandSpan.AddEvent("user", trace.WithAttributes(
				attribute.Bool("authenticated", upCtx.Organization != ""),
			))

			break
		}
	}

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

// RootCommandRunner is a struct that implements the CommandRunner interface,
// used by subcommands to run other `up` commands programmatically.
type RootCommandRunner struct{}

var _ runner.CommandRunner = &RootCommandRunner{}

// RunCommand runs the `up` command with the given arguments.
func (r *RootCommandRunner) RunCommand(args []string) error {
	parser, err := kong.New(&cli{})
	if err != nil {
		return err
	}
	ctx, err := parser.Parse(args)
	if err != nil {
		return err
	}
	return ctx.Run()
}

type cli struct {
	Format config.Format    `default:"default"           enum:"default,json,yaml"    help:"Format for get/list commands. Can be: json, yaml, default" name:"format"`
	Quiet  config.QuietFlag `help:"Suppress all output." name:"quiet"                short:"q"`
	Pretty *bool            `env:"PRETTY"                help:"Pretty print output." name:"pretty"`
	DryRun bool             `help:"dry-run output."      name:"dry-run"`

	// Manage Upbound Resources
	Organization organization.Cmd `aliases:"org"  cmd:""                           group:"Manage Upbound Resources"             help:"Interact with Upbound organizations." name:"organization"`
	Token        token.Cmd        `cmd:""         group:"Manage Upbound Resources" help:"Interact with personal access tokens." name:"token"`
	Team         team.Cmd         `cmd:""         group:"Manage Upbound Resources" help:"Interact with teams."                  name:"team"`
	Robot        robot.Cmd        `cmd:""         group:"Manage Upbound Resources" help:"Interact with robots."                 name:"robot"`
	Repository   repository.Cmd   `aliases:"repo" cmd:""                           group:"Manage Upbound Resources"             help:"Interact with repositories."          name:"repository"`
	Space        space.Cmd        `cmd:""         group:"Manage Upbound Resources" help:"Interact with Spaces."`
	Group        group.Cmd        `cmd:""         group:"Manage Upbound Resources" help:"Interact with groups inside Spaces."`
	ControlPlane controlplane.Cmd `aliases:"ctp"  cmd:""                           group:"Manage Upbound Resources"             help:"Interact with control planes."        name:"controlplane"`
	UXP          uxp.Cmd          `cmd:""         group:"Manage Upbound Resources" help:"Interact with UXP."`

	// Develop with Crossplane
	Project     project.Cmd     `cmd:""        group:"Develop with Crossplane" help:"Manage Upbound development projects."`
	Example     example.Cmd     `cmd:""        group:"Develop with Crossplane" help:"Manage Claim(XRC) or Composite Resource(XR)."`
	Dependency  dependency.Cmd  `aliases:"dep" cmd:""                          group:"Develop with Crossplane"                                            help:"Manage configuration dependencies."`
	XRD         xrd.Cmd         `cmd:""        group:"Develop with Crossplane" help:"Manage XRDs from Composite Resources(XR) or Claims(XRC)."`
	Composition composition.Cmd `cmd:""        group:"Develop with Crossplane" help:"Manage Compositions."`
	Function    function.Cmd    `cmd:""        group:"Develop with Crossplane" help:"Manage Functions."`
	Operation   operation.Cmd   `cmd:""        group:"Develop with Crossplane" help:"Manage Operations."`
	Test        test.Cmd        `cmd:""        group:"Develop with Crossplane" help:"Manage and run tests for projects."`
	XPKG        xpkg.Cmd        `cmd:""        group:"Develop with Crossplane" help:"Deprecated. Please migrate to up project or use the crossplane CLI." maturity:"deprecated"`
	XPLS        xpls.Cmd        `cmd:""        group:"Develop with Crossplane" help:"Start xpls language server."`

	// Configure up
	Completion kongplete.InstallCompletions `cmd:"" group:"Configure up" help:"Generate shell autocompletions"`
	Config     configcmd.Cmd                `cmd:"" group:"Configure up" help:"Manage global configuration settings."`
	Ctx        ctx.Cmd                      `cmd:"" group:"Configure up" help:"Select an Upbound kubeconfig context."`
	Help       helpCmd                      `cmd:"" group:"Configure up" help:"Show help."`
	License    licenseCmd                   `cmd:"" group:"Configure up" help:"Show license information."`
	Profile    profile.Cmd                  `cmd:"" group:"Configure up" help:"Manage configuration profiles."`
	Login      login.LoginCmd               `cmd:"" group:"Configure up" help:"Login to Upbound. Will attempt to launch a web browser by default. Use --username and --password flags for automations."`
	Logout     login.LogoutCmd              `cmd:"" group:"Configure up" help:"Logout of Upbound."`
	Version    v.Cmd                        `cmd:"" group:"Configure up" help:"Show current version."`

	// Alpha contains alpha commands, which we hide in the top-level help and
	// documentation. These commands should be documented in the product docs
	// for their relevant features.
	Alpha alpha `cmd:"" help:"Alpha features. Commands may be removed in future releases." hidden:""`

	GenerateDocs docsCmd `cmd:"" help:"Generate documentation in YAML format." hidden:""`

	// We set this so we can print shutdown errors if otel is in debug mode.
	// Bit of hack :/ sorry
	otelDebug bool `kong:"-"`
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
	// ControlPlane has two alpha commands: `simulate` and `simulation`.
	ControlPlane controlplane.Cmd `aliases:"ctp" cmd:""                                                                                                                          help:"Interact with control planes." hidden:""        name:"controlplane"`
	Migration    migration.Cmd    `cmd:""        help:"Migrate control planes to Upbound Managed Control Planes. Deprecated: use \"up controlplane migration\" command instead." hidden:""`
	Trace        tracecmd.Cmd     `cmd:""        help:"Trace a Crossplane resource."                                                                                             hidden:""                            maturity:"alpha"`
	Query        query.QueryCmd   `cmd:""        help:"Query objects in one or many control planes."                                                                             hidden:""                            maturity:"alpha"`
	Get          query.GetCmd     `cmd:""        help:"Get objects in the current control plane."                                                                                hidden:""                            maturity:"alpha"`
	// Xpkg has one alpha command: `append`.
	Xpkg xpkg.Cmd `cmd:"" help:"Manage Crossplane packages." hidden:""`
}

//go:embed help.md
var helpDescription string

func main() {
	c := cli{}

	parser := kong.Must(&c,
		kong.Name("up"),
		kong.Help(helpPrinter),
	)

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
	kongCtx.Model.Detail = helpDescription

	// Ensure OTEL client and spans are properly shut down
	defer func() {
		// End the command span first
		if globalCommandSpan != nil {
			globalCommandSpan.End()
		}

		if globalOTELClient != nil {
			// We allow 1 seconds to export remaining spans. We might need to increase this.
			shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			err := globalOTELClient.Shutdown(shutdownCtx)
			if err != nil {
				if c.otelDebug {
					pterm.Error.Printfln("Error shutting down OTEL client: %v", err)
				}
			}
		}
	}()

	// Execute the command
	err = kongCtx.Run()

	// Record error in span if command failed
	if err != nil && globalCommandSpan != nil {
		globalCommandSpan.RecordError(err)
		globalCommandSpan.SetStatus(codes.Error, err.Error())
	}

	kongCtx.FatalIfErrorf(err)
}
