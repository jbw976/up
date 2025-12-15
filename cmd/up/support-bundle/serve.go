// Copyright 2025 Upbound Inc.
// All rights reserved

package supportbundle

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/pterm/pterm"

	"github.com/upbound/up/internal/supportbundle/serve"
	"github.com/upbound/up/internal/upterm"

	_ "embed"
)

//go:embed help/serve.md
var serveHelp string

// serveCmd serves support bundle files over HTTP for live viewing.
type serveCmd struct {
	Path           string `arg:""                                default:"."                                                    help:"Path to support bundle directory or archive."`
	Host           string `default:"127.0.0.1"                   help:"Host to serve on."                                       name:"host"`
	Port           int    `default:"0"                           help:"Port to serve on. 0 means a random port will be chosen." name:"port"`
	KubeconfigPath string `default:"./support-bundle-kubeconfig" help:"Where to write the kubeconfig file."                     short:"k"`
	EnvtestArch    string `default:""                            help:"Arch value for Kubernetes API Server assets."            short:"a"`
	Debug          bool   `help:"Enable debug output."           short:"d"`
}

// Help prints help.
func (c *serveCmd) Help() string {
	return serveHelp
}

// Run executes the support bundle serve command.
func (c *serveCmd) Run(ctx context.Context) error {
	if c.Debug {
		pterm.EnableDebugMessages()
	}

	spinner := upterm.NewSuccessSpinner(fmt.Sprintf("Loading support bundle from: %s", c.Path))
	spinner.Start()

	addr := net.JoinHostPort(c.Host, strconv.Itoa(c.Port))

	opts := serve.Options{
		BundlePath:     c.Path,
		Addr:           addr,
		KubeconfigPath: c.KubeconfigPath,
		EnvtestArch:    c.EnvtestArch,
		Debug:          c.Debug,
		Debugf: func(format string, args ...any) {
			pterm.Debug.Printfln(format, args...)
		},
		//nolint:forbidigo // It's a CLI.
		OnServerReady: func(serverURL, kubeconfigPath string) {
			spinner.Success()
			fmt.Printf("\nServing support bundle at: %s\n", serverURL)
			fmt.Printf("\n")
			fmt.Printf("KUBECONFIG=%s\n", kubeconfigPath)
			fmt.Printf("\n")
			fmt.Printf("You can now view the support bundle content using kubectl or k9s:\n")
			fmt.Printf("\n")
			fmt.Printf("  kubectl --kubeconfig=%s get pods --all-namespaces\n", kubeconfigPath)
			fmt.Printf("\n")
			fmt.Printf("Press Ctrl+C to stop the server\n")
			fmt.Printf("\n")
		},
	}

	err := serve.Start(ctx, opts)
	if err != nil {
		spinner.Fail()
		return err
	}

	return nil
}
