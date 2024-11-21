// Copyright 2022 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package user contains commands for working with users in an organization.
package user

import (
	"github.com/alecthomas/kong"

	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up/internal/upbound"
)

// AfterApply constructs and binds a robots client to any subcommands
// that have Run() methods that receive it.
func (c *Cmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	cfg, err := upCtx.BuildSDKConfig()
	if err != nil {
		return err
	}
	kongCtx.Bind(organizations.NewClient(cfg))
	return nil
}

// Cmd contains commands for managing organization users.
type Cmd struct {
	List   listCmd   `cmd:"" help:"List users of an organization."`
	Invite inviteCmd `cmd:"" help:"Invite a user to the organization."`
	Remove removeCmd `cmd:"" help:"Remove a member from the organization."`
}
