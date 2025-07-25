// Copyright 2025 Upbound Inc.
// All rights reserved

// Package version contains version cmd
package version

import (
	"context"
	"flag"
	"fmt"
	"runtime"

	"github.com/alecthomas/kong"
	"k8s.io/client-go/kubernetes"

	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/internal/version"
)

const (
	errKubeConfig      = "failed to get kubeconfig"
	errCreateK8sClient = "failed to connect to cluster"

	errGetCrossplaneVersion = "unable to get crossplane version. Is your kubecontext pointed at a control plane?"
	errGetSpacesVersion     = "unable to get spaces version. Is your kubecontext pointed at a Space?"
)

const (
	versionUnknown  = "unknown"
	versionTemplate = `{{with .Client -}}
Client:
  Version:	{{.Version}}
  Go Version:	{{.GoVersion}}
  Git Commit: 	{{.GitCommit}}
  OS/Arch:	{{.OS}}/{{.Arch}}
{{- end}}

{{- if ne .Server nil}}{{with .Server}}
Server:
  Crossplane Version:	{{.CrossplaneVersion}}
  Spaces Controller Version:	{{.SpacesControllerVersion}}
{{- end}}{{- end}}`
)

type clientVersion struct {
	Arch      string `json:"arch,omitempty"      yaml:"arch,omitempty"`
	GitCommit string `json:"gitCommit,omitempty" yaml:"gitCommit,omitempty"`
	GoVersion string `json:"goVersion,omitempty" yaml:"goVersion,omitempty"`
	OS        string `json:"os,omitempty"        yaml:"os,omitempty"`
	Version   string `json:"version,omitempty"   yaml:"version,omitempty"`
}

type serverVersion struct {
	CrossplaneVersion       string `json:"crossplaneVersion,omitempty"       yaml:"crossplaneVersion,omitempty"`
	SpacesControllerVersion string `json:"spacesControllerVersion,omitempty" yaml:"spacesControllerVersion,omitempty"`
}

type versionInfo struct {
	Client clientVersion  `json:"client"           yaml:"client"`
	Server *serverVersion `json:"server,omitempty" yaml:"server,omitempty"`
}

// Cmd is the `up version` command.
type Cmd struct {
	Client bool `env:"" help:"If true, shows client version only (no server required)." json:"client,omitempty"`

	// Common Upbound API configuration
	Flags upbound.Flags `embed:""`
}

// AfterApply parses flags and applies defaults.
func (c *Cmd) AfterApply(kongCtx *kong.Context) error {
	upCtx, err := upbound.NewFromFlags(c.Flags)
	if err != nil {
		return err
	}
	upCtx.SetupLogging()

	kongCtx.Bind(upCtx)

	return nil
}

// BeforeApply sets default values and parses flags.
func (c *Cmd) BeforeApply() error {
	flag.BoolVar(&c.Client, "client", false, "If true, shows client version only (no server required).")
	flag.Parse()
	return nil
}

// Help returns help for the command.
func (c *Cmd) Help() string {
	return `
Options:
  --client=false:
  If true, shows client version only (no server required).

Usage:
  up version [flags] [options]
`
}

func (c *Cmd) buildVersionInfo(ctx context.Context, kongCtx *kong.Context, upCtx *upbound.Context) (v versionInfo) {
	v.Client = clientVersion{
		Version:   version.Version(),
		Arch:      runtime.GOARCH,
		OS:        runtime.GOOS,
		GoVersion: runtime.Version(),
		GitCommit: version.GitCommit(),
	}

	if c.Client {
		return v
	}

	context, _, _, ok := upCtx.GetCurrentContext()
	if !ok || context == nil {
		fmt.Fprintln(kongCtx.Stderr, errKubeConfig) //nolint:errcheck // Debug logging.
		return v
	}

	rest, err := upCtx.GetKubeconfig()
	if err != nil {
		fmt.Fprintln(kongCtx.Stderr, errCreateK8sClient) //nolint:errcheck // Debug logging.
		return v
	}

	clientset, err := kubernetes.NewForConfig(rest)
	if err != nil {
		fmt.Fprintln(kongCtx.Stderr, errCreateK8sClient) //nolint:errcheck // Debug logging.
		return v
	}

	v.Server = &serverVersion{}
	v.Server.CrossplaneVersion, err = FetchCrossplaneVersion(ctx, *clientset)
	if err != nil {
		fmt.Fprintln(kongCtx.Stderr, errGetCrossplaneVersion) //nolint:errcheck // Debug logging.
	}
	if v.Server.CrossplaneVersion == "" {
		v.Server.CrossplaneVersion = versionUnknown
	}

	v.Server.SpacesControllerVersion, err = FetchSpacesVersion(ctx, context, *clientset)
	if err != nil {
		fmt.Fprintln(kongCtx.Stderr, errGetSpacesVersion) //nolint:errcheck // Debug logging.
	}
	if v.Server.SpacesControllerVersion == "" {
		v.Server.SpacesControllerVersion = versionUnknown
	}

	return v
}

// Run is the implementation of the command.
func (c *Cmd) Run(ctx context.Context, kongCtx *kong.Context, upCtx *upbound.Context, printer upterm.Printer) error {
	v := c.buildVersionInfo(ctx, kongCtx, upCtx)

	return printer.PrintTemplate(v, versionTemplate)
}
