// Copyright 2025 Upbound Inc.
// All rights reserved

// Package main is the up CLI.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/alecthomas/kong"
	"github.com/willabides/kongplete"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/term"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/upbound/up/cmd/up/composition"
	configcmd "github.com/upbound/up/cmd/up/config"
	"github.com/upbound/up/cmd/up/controlplane"
	"github.com/upbound/up/cmd/up/ctx"
	"github.com/upbound/up/cmd/up/dependency"
	"github.com/upbound/up/cmd/up/example"
	"github.com/upbound/up/cmd/up/function"
	"github.com/upbound/up/cmd/up/group"
	"github.com/upbound/up/cmd/up/login"
	"github.com/upbound/up/cmd/up/operation"
	"github.com/upbound/up/cmd/up/organization"
	"github.com/upbound/up/cmd/up/profile"
	"github.com/upbound/up/cmd/up/project"
	"github.com/upbound/up/cmd/up/query"
	"github.com/upbound/up/cmd/up/repository"
	"github.com/upbound/up/cmd/up/resource"
	"github.com/upbound/up/cmd/up/robot"
	"github.com/upbound/up/cmd/up/runner"
	"github.com/upbound/up/cmd/up/space"
	supportbundle "github.com/upbound/up/cmd/up/support-bundle"
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
	var pretty bool
	if c.Pretty != nil {
		pretty = *c.Pretty
	} else {
		pretty = term.IsTerminal(int(os.Stdout.Fd()))
	}

	stdout := kongCtx.Stdout
	resultOut := kongCtx.Stdout
	if c.Quiet || c.Silent {
		stdout = io.Discard
	}
	if c.Silent {
		resultOut = io.Discard
	}

	// Construct a printer and bind it to the context. Commands should take the
	// printer as an arg and use it for all output.
	printer := upterm.NewPrinter(stdout, resultOut, c.Format, pretty)
	kongCtx.BindTo(printer, (*upterm.Printer)(nil))
	kongCtx.BindTo(printer, (*upterm.ResultPrinter)(nil))
	kongCtx.BindTo(printer, (*upterm.SpinnerPrinter)(nil))

	kongCtx.BindTo(&RootCommandRunner{}, (*runner.CommandRunner)(nil))

	// Initialize and bind OpenTelemetry client
	f := sync.OnceFunc(func() {
		if err := c.initOTEL(kongCtx); err != nil {
			fmt.Printf("Error initializing telemetry: %v\n", err) //nolint:forbidigo // Last resort.
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
	Format config.Format `default:"default"                                                                enum:"default,json,yaml"    help:"Format for get/list commands. Can be: json, yaml, default" name:"format"`
	Quiet  bool          `help:"Suppress all informational output. Command results will still be printed." name:"quiet"                short:"q"`
	Silent bool          `help:"Suppress all output."`
	Pretty *bool         `env:"PRETTY"                                                                     help:"Pretty print output." name:"pretty"`

	// Manage Upbound Resources
	Organization  organization.Cmd  `aliases:"org"  cmd:""                           group:"Manage Upbound Resources"                                         help:"Interact with Upbound organizations." name:"organization"`
	Token         token.Cmd         `cmd:""         group:"Manage Upbound Resources" help:"Interact with personal access tokens."                             name:"token"`
	Team          team.Cmd          `cmd:""         group:"Manage Upbound Resources" help:"Interact with teams."                                              name:"team"`
	Robot         robot.Cmd         `cmd:""         group:"Manage Upbound Resources" help:"Interact with robots."                                             name:"robot"`
	Repository    repository.Cmd    `aliases:"repo" cmd:""                           group:"Manage Upbound Resources"                                         help:"Interact with repositories."          name:"repository"`
	Space         space.Cmd         `cmd:""         group:"Manage Upbound Resources" help:"Interact with Spaces."`
	Group         group.Cmd         `cmd:""         group:"Manage Upbound Resources" help:"Interact with groups inside Spaces."`
	ControlPlane  controlplane.Cmd  `aliases:"ctp"  cmd:""                           group:"Manage Upbound Resources"                                         help:"Interact with control planes."        name:"controlplane"`
	UXP           uxp.Cmd           `cmd:""         group:"Manage Upbound Resources" help:"Interact with UXP."`
	SupportBundle supportbundle.Cmd `cmd:""         group:"Manage Upbound Resources" help:"Collect support bundles for troubleshooting."                      name:"support-bundle"`
	Resource      resource.Cmd      `cmd:""         group:"Manage Upbound Resources" help:"Gather information about resources in a cluster or control plane."`

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
	ControlPlane controlplane.Cmd `aliases:"ctp" cmd:""                                              help:"Interact with control planes." hidden:""        name:"controlplane"`
	Trace        tracecmd.Cmd     `cmd:""        help:"Trace a Crossplane resource."                 hidden:""                            maturity:"alpha"`
	Query        query.QueryCmd   `cmd:""        help:"Query objects in one or many control planes." hidden:""                            maturity:"alpha"`
	Get          query.GetCmd     `cmd:""        help:"Get objects in the current control plane."    hidden:""                            maturity:"alpha"`
	// Xpkg has one alpha command: `append`.
	Xpkg xpkg.Cmd `cmd:"" help:"Manage Crossplane packages." hidden:""`
}

//go:embed help.md
var helpDescription string

func main() {
	c := cli{}

	// Ensure OTEL client and spans are properly shut down before exit.
	exit := func(code int) {
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
					fmt.Printf("Error shutting down OTEL client: %v\n", err) //nolint:forbidigo // Deubg output.
				}
			}
		}

		os.Exit(code)
	}
	// If there's an error, kong will call exit with a non-zero code and defers
	// won't run. If there's no error, make sure our telemetry shutdown runs.
	defer exit(0)

	parser := kong.Must(&c,
		kong.Name("up"),
		kong.Help(helpPrinter),
		kong.Exit(exit),
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

	// For help invocations, we don't end the span or wait for the otel client,
	// since telemetry on help isn't interesting.
	if len(os.Args) == 1 {
		_, err := parser.Parse([]string{"--help"})
		parser.FatalIfErrorf(err)
		return
	}

	// If the command fails (during parse or execution) we mark the span with an
	// error status. We don't send the error message since it may contain
	// sensitive values (e.g., file paths), but include the error's (unwrapped)
	// type in case it's interesting.

	kongCtx, err := parser.Parse(os.Args[1:])
	if err != nil && globalCommandSpan != nil {
		globalCommandSpan.SetStatus(codes.Error, fmt.Sprintf("%T", unwrap(err)))
	}
	parser.FatalIfErrorf(err)
	kongCtx.BindTo(context.Background(), (*context.Context)(nil))
	kongCtx.Model.Detail = helpDescription

	// Execute the command
	err = kongCtx.Run()

	if err != nil && globalCommandSpan != nil {
		globalCommandSpan.SetStatus(codes.Error, fmt.Sprintf("%T", unwrap(err)))
	}

	kongCtx.FatalIfErrorf(err)
}

// unwrap unwraps an error as far as possible. Unlike `errors.Unwrap` it will
// never return nil for a non-nil error.
func unwrap(err error) error {
	for errors.Unwrap(err) != nil {
		err = errors.Unwrap(err)
	}

	return err
}
