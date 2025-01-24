// Copyright 2025 Upbound Inc.
// All rights reserved

package config

// Cmd contains commands for configuring Upbound Profiles.
type Cmd struct {
	Set   setCmd   `cmd:"" help:"Set base configuration key, value pair in the Upbound Profile."`
	UnSet unsetCmd `cmd:"" help:"Unset base configuration key, value pair in the Upbound Profile." name:"unset"`
}
