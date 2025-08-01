// Copyright 2025 Upbound Inc.
// All rights reserved

package upbound

// RequiresContext can be embedded into a command struct to indicate that the
// command requires an upbound context. The main package will construct the
// context appropriately.
type RequiresContext struct {
	Flags Flags `embed:""`
}

// GetUpboundContext returns a context constructed from flags.
func (r RequiresContext) GetUpboundContext() (*Context, error) {
	upCtx, err := NewFromFlags(r.Flags)
	if err != nil {
		return nil, err
	}
	upCtx.SetupLogging()
	return upCtx, nil
}

// RequiresContextAllowMissingProfile can be embedded into a command struct to
// indicate that the command requires an upbound context but does not require a
// valid profile to be present. The main package will construct the context
// appropriately.
type RequiresContextAllowMissingProfile struct {
	Flags Flags `embed:""`
}

// GetUpboundContext returns a context constructed from flags.
func (r RequiresContextAllowMissingProfile) GetUpboundContext() (*Context, error) {
	upCtx, err := NewFromFlags(r.Flags, AllowMissingProfile())
	if err != nil {
		return nil, err
	}
	upCtx.SetupLogging()
	return upCtx, nil
}

// ContextRequirer is implemented by RequiresContext, allowing the main package to
// construct contexts for commands.
type ContextRequirer interface {
	GetUpboundContext() (*Context, error)
}

// Assert that our requirements implement the right interface.
var (
	_ ContextRequirer = (*RequiresContext)(nil)
	_ ContextRequirer = (*RequiresContextAllowMissingProfile)(nil)
)
