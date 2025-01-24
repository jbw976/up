// Copyright 2025 Upbound Inc.
// All rights reserved

package billing

type Cmd struct {
	Export exportCmd `cmd:"" help:"Export a billing report for submission to Upbound."`
}
