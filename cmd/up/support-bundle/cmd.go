// Copyright 2025 Upbound Inc.
// All rights reserved

// Package supportbundle handles support bundle commands
package supportbundle

// Cmd contains commands for collecting support bundles.
type Cmd struct {
	Collect  collectCmd  `cmd:"" help:"Collect a support bundle from the current kube context."`
	Template templateCmd `cmd:"" help:"Output the default SupportBundle YAML configuration template."`
}
