// Copyright 2025 Upbound Inc.
// All rights reserved

package supportbundle

import (
	"context"
	"fmt"

	"github.com/pterm/pterm"

	"github.com/upbound/up/internal/supportbundle/serve"
	"github.com/upbound/up/internal/upterm"

	_ "embed"
)

//go:embed help/serve.md
var serveHelp string

// serveCmd serves support bundle files over HTTP for live viewing.
type serveCmd struct {
	Path           string `arg:""                                default:"."                                              help:"Path to support bundle directory or archive."`
	Host           string `default:"localhost:8080"              help:"Host and port to serve on (e.g., localhost:8080)."`
	KubeconfigPath string `default:"./support-bundle-kubeconfig" help:"Where to write the kubeconfig file."               short:"k"`
	EnvtestArch    string `default:""                            help:"Arch value for Kubernetes API Server assets."      short:"a"`
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

	spinner, err := upterm.CheckmarkSuccessSpinner.Start(fmt.Sprintf("Loading support bundle from: %s", c.Path))
	if err != nil {
		return err
	}

	opts := serve.Options{
		BundlePath:     c.Path,
		Host:           c.Host,
		KubeconfigPath: c.KubeconfigPath,
		EnvtestArch:    c.EnvtestArch,
		Debug:          c.Debug,
		Debugf: func(format string, args ...any) {
			pterm.Debug.Printfln(format, args...)
		},
		OnServerReady: func(serverURL, kubeconfigPath string) {
			spinner.Success()
			pterm.Println()
			pterm.Printfln("Serving support bundle at: %s", serverURL)
			pterm.Println()
			pterm.Printfln("KUBECONFIG=%s", kubeconfigPath)
			pterm.Println()
			pterm.Printfln("You can now view the support bundle content using kubectl or k9s:")
			pterm.Printfln("  kubectl --kubeconfig=%s get pods --all-namespaces", kubeconfigPath)
			pterm.Println()
			pterm.Printfln("Press Ctrl+C to stop the server")
		},
	}

	err = serve.Start(ctx, opts)
	if err != nil {
		spinner.Fail()
		return err
	}

	return nil
}
