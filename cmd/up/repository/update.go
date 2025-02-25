// Copyright 2025 Upbound Inc.
// All rights reserved

package repository

import (
	"context"
	"fmt"

	"github.com/pterm/pterm"

	"github.com/upbound/up-sdk-go/service/repositories"
	"github.com/upbound/up/internal/input"
	"github.com/upbound/up/internal/upbound"
)

// BeforeApply sets default values for the command before assignment and validation.
func (u *updateCmd) BeforeApply() error {
	u.prompter = input.NewPrompter()
	return nil
}

// AfterApply accepts user input by default to confirm the update operation.
func (u *updateCmd) AfterApply(p pterm.TextPrinter, upCtx *upbound.Context) error {
	if u.Force {
		return nil
	}
	confirm, err := u.prompter.Prompt("Updating a repository may delete Marketplace listings or force clients to require authentication. Continue? [y/n]", false)
	if err != nil {
		return err
	}

	if input.InputYes(confirm) {
		p.Printfln("Updating repository %s/%s with private=%t and publish=%t", upCtx.Organization, u.Name, u.Private, u.Publish)
		return nil
	}

	return fmt.Errorf("operation canceled")
}

// updateCmd updates an existing repository on Upbound.
type updateCmd struct {
	prompter input.Prompter

	Name string `arg:"" help:"Name of repository. Required." required:""`

	// Make all request fields explicitly required to avoid unexpected behaviour on PUT
	Private bool `help:"The desired repository visibility. Required."        required:""`
	Publish bool `help:"The desired repository publishing policy. Required." required:""`
	Force   bool `help:"Force the repository update." default:"false"` //nolint:tagalign //conflicts with a govet rule.
}

// Run executes the update command.
func (u *updateCmd) Run(ctx context.Context, p pterm.TextPrinter, rc *repositories.Client, upCtx *upbound.Context) error {
	visibility := repositories.WithPublic()
	if u.Private {
		visibility = repositories.WithPrivate()
	}

	publishPolicy := repositories.WithDraft()
	if u.Publish {
		publishPolicy = repositories.WithPublish()
	}

	if repo, _ := rc.Get(ctx, upCtx.Organization, u.Name); repo == nil {
		return fmt.Errorf("the repository %s does not exist. use `create` instead", u.Name)
	}

	if err := rc.CreateOrUpdateWithOptions(ctx, upCtx.Organization, u.Name, visibility, publishPolicy); err != nil {
		return err
	}
	p.Printfln("%s/%s updated", upCtx.Organization, u.Name)
	return nil
}
