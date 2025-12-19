// Copyright 2025 Upbound Inc.
// All rights reserved

package supportbundle

import (
	"context"
	"fmt"
	"net"
	"strconv"

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
func (c *serveCmd) Run(ctx context.Context, pr upterm.Printer) error {
	spinner := pr.NewSuccessSpinner(fmt.Sprintf("Loading support bundle from: %s", c.Path))
	spinner.Start()

	addr := net.JoinHostPort(c.Host, strconv.Itoa(c.Port))

	opts := serve.Options{
		BundlePath:     c.Path,
		Addr:           addr,
		KubeconfigPath: c.KubeconfigPath,
		EnvtestArch:    c.EnvtestArch,
		Debug:          c.Debug,
		Debugf: func(format string, args ...any) {
			if c.Debug {
				pr.Printfln(format, args...)
			}
		},
		OnServerReady: func(serverURL, kubeconfigPath string) {
			spinner.Success()
			pr.Println()
			pr.Printfln("Serving support bundle at: %s", serverURL)
			pr.Println()
			pr.Printfln("KUBECONFIG=%s", kubeconfigPath)
			pr.Println("")
			pr.Printfln("You can now view the support bundle content using kubectl or k9s:")
			pr.Println()
			pr.Printfln("  kubectl --kubeconfig=%s get pods --all-namespaces", kubeconfigPath)
			pr.Println()
			pr.Printfln("Press Ctrl+C to stop the server")
			pr.Println()
		},
	}

	err := serve.Start(ctx, opts)
	if err != nil {
		spinner.Fail()
		return err
	}

	return nil
}
