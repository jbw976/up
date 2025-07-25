// Copyright 2025 Upbound Inc.
// All rights reserved

package space

import (
	"github.com/alecthomas/kong"

	"github.com/upbound/up/cmd/up/space/billing"
	"github.com/upbound/up/internal/feature"
	"github.com/upbound/up/internal/upbound"
)

const (
	spacesChart = "spaces"

	defaultRegistry = "xpkg.upbound.io/spaces-artifacts"
)

// BeforeReset is the first hook to run.
func (c *Cmd) BeforeReset(p *kong.Path, maturity feature.Maturity) error {
	return feature.HideMaturity(p, maturity)
}

// Cmd contains commands for interacting with spaces.
type Cmd struct {
	Connect    connectCmd    `aliases:"attach" cmd:"" help:"Connect an Upbound Space to the Upbound web console."`
	Disconnect disconnectCmd `aliases:"detach" cmd:"" help:"Disconnect an Upbound Space from the Upbound web console."`

	Destroy destroyCmd `cmd:"" help:"Remove the Upbound Spaces deployment."`
	Init    initCmd    `cmd:"" help:"Initialize an Upbound Spaces deployment."`
	List    listCmd    `cmd:"" help:"List all accessible spaces in Upbound."`
	Mirror  mirrorCmd  `cmd:"" help:"Managing the mirroring of artifacts to local storage or private container registries."`
	Upgrade upgradeCmd `cmd:"" help:"Upgrade the Upbound Spaces deployment."`

	Billing billing.Cmd `cmd:""`
}

// overrideRegistry is a common function that takes the candidate registry,
// compares that against the default registry and if different overrides
// that property in the params map.
func overrideRegistry(candidate string, params map[string]any) {
	// NOTE(tnthornton) this is unfortunately brittle. If the helm chart values
	// property changes, this won't necessarily account for that.
	if candidate != defaultRegistry {
		params["registry"] = candidate
	}
}

func ensureAccount(upCtx *upbound.Context, params map[string]any) {
	// If the account name was explicitly set via helm flags, keep it.
	_, ok := params["account"]
	if ok {
		return
	}

	// Get the account from the active profile if it's set.
	if upCtx.Organization != "" {
		params["account"] = upCtx.Organization
		return
	}

	// Fall back to the default if we didn't find an account name
	// elsewhere. Spaces created with the default can't be attached to the
	// console, so this is not ideal, but they can be used in disconnected mode.
	params["account"] = defaultAcct
}
